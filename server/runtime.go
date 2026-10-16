package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/logging"
	"github.com/asciimoo/hister/server/diagnostics"
	"github.com/asciimoo/hister/server/extractor"
	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/server/metrics"
	"github.com/asciimoo/hister/server/model"

	"github.com/rs/zerolog/log"
)

// serverRuntime owns a single running generation. Global model, extractor,
// session, and asset state may only change once that generation is drained.
type serverRuntime struct {
	mu        sync.Mutex
	reloadMu  sync.Mutex
	current   *runtimeGeneration
	paused    bool
	ctx       context.Context
	overrides []func(*config.Config) error
	serving   *runtimeListener
	failures  chan error
}

type runtimeGeneration struct {
	cfg          *config.Config
	idx          *indexer.Indexer
	db           *sql.DB
	handler      http.Handler
	active       int
	drained      chan struct{}
	closeOnce    sync.Once
	workers      sync.WaitGroup
	queueWorker  sync.WaitGroup
	stop         context.CancelFunc
	stopQueue    context.CancelFunc
	sockets      context.Context
	closeSockets context.CancelFunc
	metrics      *metrics.Metrics
}

type runtimeListener struct {
	listener net.Listener
	server   *http.Server
	done     chan struct{}
}

type runtimeRequestKey struct{}

type runtimeRequest struct {
	runtime    *serverRuntime
	generation *runtimeGeneration
	sockets    context.Context
	finish     func()
}

func (r *serverRuntime) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	if r.paused || r.current == nil || r.ctx.Err() != nil {
		r.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		http.Error(w, "server is reloading", http.StatusServiceUnavailable)
		return
	}
	g := r.current
	if g.active == 0 {
		g.drained = make(chan struct{})
	}
	g.active++
	finish := sync.OnceFunc(func() {
		r.mu.Lock()
		g.active--
		if g.active == 0 {
			close(g.drained)
		}
		r.mu.Unlock()
	})
	lease := &runtimeRequest{runtime: r, generation: g, sockets: g.sockets, finish: finish}
	r.mu.Unlock()
	defer lease.finish()
	g.handler.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), runtimeRequestKey{}, lease)))
}

func (r *serverRuntime) startListener(listener net.Listener) *runtimeListener {
	// Leave WriteTimeout unset so reindexing and other long running handlers
	// can finish their responses after the request body has been read.
	s := &runtimeListener{listener: listener, server: &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		IdleTimeout:       time.Minute,
	}, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		err := s.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			select {
			case r.failures <- err:
			default:
			}
		}
	}()
	return s
}

func (r *serverRuntime) generation(cfg *config.Config, idx *indexer.Indexer) (*runtimeGeneration, error) {
	db, err := model.DB.DB()
	if err != nil {
		return nil, err
	}
	if err := parseStaticFiles(cfg.BasePathPrefix()); err != nil {
		return nil, err
	}
	var watcher *files.Watcher
	if len(cfg.Indexer.Directories) > 0 {
		watcher, err = files.NewDirectoryWatcher(cfg.Indexer.Directories)
		if err != nil {
			return nil, err
		}
	}
	sessionStore = newSessionStore(cfg.SecretKey(), cfg.BaseURL(""), sessionMaxAge)
	ctx, cancel := context.WithCancel(r.ctx)
	g := &runtimeGeneration{cfg: cfg, idx: idx, db: db, stop: cancel}
	g.drained = make(chan struct{})
	close(g.drained)
	g.sockets, g.closeSockets = context.WithCancel(r.ctx)
	g.handler = withLogging(registerEndpoints(cfg, idx, r.overrides...))
	if cfg.Server.Metrics {
		g.metrics = metrics.New(ctx, idx)
		idx.SetMetrics(g.metrics)
	}
	if len(cfg.Indexer.Directories) > 0 {
		queue := idx.NewFileIndexQueue()
		// Stop producers before the queue so pending file removals and writes
		// finish before the old index and database are closed.
		queueCtx, cancelQueue := context.WithCancel(context.Background())
		g.stopQueue = cancelQueue
		g.queueWorker.Go(func() {
			if err := queue.Run(queueCtx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error().Err(err).Msg("File index queue failed")
			}
		})
		g.workers.Go(func() { queue.EnqueueAll(cfg.Indexer.Directories) })
		g.workers.Go(func() {
			defer func() {
				if err := watcher.Close(); err != nil {
					log.Error().Err(err).Msg("Failed to close file watcher")
				}
			}()
			err := watcher.Run(ctx, func(path string) {
				userID, err := files.FindDirUser(cfg.Indexer.Directories, path)
				if err != nil || (userID != 0 && !cfg.App.UserHandling) {
					log.Error().Str("path", path).Msg("Failed to resolve file owner")
					return
				}
				queue.EnqueueIndex(path, userID)
			}, queue.EnqueueDelete)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Error().Err(err).Msg("File watcher failed")
			}
		})
	}
	return g, nil
}

func (g *runtimeGeneration) close() {
	g.closeOnce.Do(func() {
		g.closeSockets()
		g.stop()
		g.workers.Wait()
		if g.stopQueue != nil {
			g.stopQueue()
		}
		g.queueWorker.Wait()
		if g.metrics != nil {
			g.metrics.Stop()
		}
		g.idx.Close()
		if err := g.db.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
	})
}

func (r *serverRuntime) open(cfg *config.Config, registry *extractor.Registry) (*runtimeGeneration, error) {
	if err := model.Init(cfg); err != nil {
		closeRuntimeDatabase()
		return nil, errors.New("cannot open database")
	}
	extractor.DefaultRegistry().Replace(registry)
	idx, err := indexer.New(cfg)
	if err != nil {
		closeRuntimeDatabase()
		return nil, errors.New("cannot open search index")
	}
	if cfg.SemanticSearch.Enable && !idx.SemanticSearchEnabled() {
		idx.Close()
		closeRuntimeDatabase()
		return nil, errors.New("cannot initialize semantic search")
	}
	for _, check := range diagnostics.Index(cfg, idx) {
		if check.Status != "ok" {
			log.Warn().Msg(check.Message)
		}
	}
	g, err := r.generation(cfg, idx)
	if err != nil {
		idx.Close()
		closeRuntimeDatabase()
	}
	return g, err
}

func closeRuntimeDatabase() {
	if model.DB != nil {
		if db, err := model.DB.DB(); err == nil {
			_ = db.Close()
		}
	}
}

// reload is called after authorization. Release this request's generation
// lease before draining; the rest of this handler only writes its response.
func (r *serverRuntime) reload(req *http.Request, lease *runtimeRequest, next *config.Config) (int, error) {
	lease.finish()
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	r.mu.Lock()
	old := r.current
	stale := r.paused || old != lease.generation || r.ctx.Err() != nil
	r.mu.Unlock()
	if stale {
		return http.StatusConflict, errors.New("server configuration changed; retry reload")
	}
	registry, err := diagnostics.Extractors(next)
	if err != nil {
		return http.StatusBadRequest, errors.New("invalid extractor configuration; active configuration retained")
	}
	patterns := make([]string, 0, len(next.SensitiveContentPatterns))
	for _, pattern := range next.SensitiveContentPatterns {
		patterns = append(patterns, pattern)
	}
	if _, err := regexp.Compile("(" + strings.Join(patterns, "|") + ")"); err != nil {
		return http.StatusBadRequest, errors.New("invalid sensitive content patterns; active configuration retained")
	}
	if next.App.Directory != old.cfg.App.Directory {
		if err := next.PrepareDataDirectory(); err != nil {
			return http.StatusBadRequest, errors.New("cannot initialize new data directory; active configuration retained")
		}
	}
	var replacement net.Listener
	if next.Server.Address != old.cfg.Server.Address {
		replacement, err = newListener(next.Server)
		if err != nil {
			return http.StatusBadRequest, errors.New("cannot bind new server.address; active configuration retained")
		}
		defer func() {
			if replacement != nil {
				_ = replacement.Close()
			}
		}()
	}
	logChange, err := logging.Default.Prepare(next.App)
	if err != nil {
		log.Error().Err(err).Msg("Failed to prepare logging for reload")
		return http.StatusBadRequest, errors.New("cannot open app.log_file; active configuration retained")
	}
	defer func() {
		if err := logChange.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close prepared log file")
		}
	}()
	r.mu.Lock()
	r.paused = true
	old.closeSockets()
	drained := old.drained
	r.mu.Unlock()
	drainCtx, cancelDrain := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancelDrain()
	select {
	case <-drained:
	case <-drainCtx.Done():
		r.resume(old)
		return http.StatusRequestTimeout, errors.New("reload canceled while draining requests; active configuration retained")
	case <-r.ctx.Done():
		return http.StatusServiceUnavailable, errors.New("server is shutting down")
	}
	// Include rule writes that completed while this reload was draining.
	if err := next.ReadRules(); err != nil {
		r.resume(old)
		return http.StatusBadRequest, errors.New("cannot reload rules.json; active configuration retained")
	}
	old.close()
	g, err := r.open(next, registry)
	if err != nil {
		log.Error().Err(err).Msg("Runtime reload failed; restoring previous configuration")
		previousRegistry, registryErr := diagnostics.Extractors(old.cfg)
		var restoreErr error
		if registryErr == nil {
			g, restoreErr = r.open(old.cfg, previousRegistry)
		} else {
			restoreErr = registryErr
		}
		if restoreErr != nil {
			r.mu.Lock()
			r.current = nil
			r.mu.Unlock()
			select {
			case r.failures <- errors.New("reload and recovery failed; restart the server"):
			default:
			}
			return http.StatusServiceUnavailable, errors.New("reload and recovery failed; restart the server")
		}
		r.mu.Lock()
		r.current, r.paused = g, false
		r.mu.Unlock()
		return http.StatusInternalServerError, errors.New("reload failed; previous configuration restored")
	}
	if replacement != nil {
		previous := r.serving
		r.serving = r.startListener(replacement)
		replacement = nil
		previous.server.SetKeepAlivesEnabled(false)
		_ = previous.listener.Close()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := previous.server.Shutdown(ctx); err != nil {
				_ = previous.server.Close()
			}
		}()
	}
	if err := logChange.Apply(); err != nil {
		log.Warn().Err(err).Msg("Failed to close previous log file")
	}
	r.mu.Lock()
	r.current, r.paused = g, false
	r.mu.Unlock()
	log.Info().Msg("Server, database, and indexer reloaded")
	return http.StatusOK, nil
}

func (r *serverRuntime) resume(g *runtimeGeneration) {
	r.mu.Lock()
	g.sockets, g.closeSockets = context.WithCancel(r.ctx)
	r.paused = false
	r.mu.Unlock()
}

func (r *serverRuntime) close() {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	r.mu.Lock()
	r.paused = true
	g := r.current
	var drained <-chan struct{}
	if g != nil {
		g.closeSockets()
		drained = g.drained
	}
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.serving.server.Shutdown(ctx); err != nil {
		_ = r.serving.server.Close()
	}
	<-r.serving.done
	if g != nil {
		<-drained
		g.close()
	}
}

func listenRuntime(ctx context.Context, cfg *config.Config, idx *indexer.Indexer, overrides ...func(*config.Config) error) error {
	r := &serverRuntime{ctx: ctx, overrides: overrides, failures: make(chan error, 1)}
	listener, err := newListener(cfg.Server)
	if err != nil {
		idx.Close()
		closeRuntimeDatabase()
		return err
	}
	g, err := r.generation(cfg, idx)
	if err != nil {
		_ = listener.Close()
		idx.Close()
		closeRuntimeDatabase()
		return err
	}
	r.current = g
	r.serving = r.startListener(listener)
	defer r.close()
	log.Info().Str("Address", cfg.Server.Address).Str("Version", Version).Str("URL", cfg.BaseURL("/")).Msg("Starting webserver")
	select {
	case <-ctx.Done():
		return nil
	case err := <-r.failures:
		return fmt.Errorf("webserver failed: %w", err)
	}
}

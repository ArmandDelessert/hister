// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package network

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/asciimoo/hister/server/indexer"
	"github.com/asciimoo/hister/ui/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

func ListenToWebSocket(wsChan chan tea.Msg, wsDone chan struct{}) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-wsChan:
			return msg
		case <-wsDone:
			return nil
		}
	}
}

func ConnectWebSocket(wsURL, origin string, wsChan chan tea.Msg, wsDone chan struct{}) tea.Cmd {
	return func() tea.Msg {
		header := http.Header{}
		header.Set("Origin", origin)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			return model.WsDisconnectedMsg{Err: err}
		}
		go func() {
			defer conn.Close()
			for {
				select {
				case <-wsDone:
					return
				default:
					_, data, err := conn.ReadMessage()
					if err != nil {
						select {
						case wsChan <- model.WsDisconnectedMsg{Err: err}:
						case <-wsDone:
						}
						return
					}
					var res *indexer.Results
					if err := json.Unmarshal(data, &res); err != nil {
						continue
					}
					if len(res.Documents) == 0 && len(res.History) == 0 {
						res = &indexer.Results{}
					}
					select {
					case wsChan <- model.ResultsMsg{Results: res}:
					case <-wsDone:
						return
					}
				}
			}
		}()
		return model.WsConnectedMsg{Conn: conn}
	}
}

func Search(conn *websocket.Conn, wsMu *sync.Mutex, wsReady bool, q model.SearchQuery) tea.Cmd {
	return func() tea.Msg {
		if !wsReady || conn == nil {
			return nil
		}
		b, err := json.Marshal(q)
		if err != nil {
			return nil
		}
		wsMu.Lock()
		conn.WriteMessage(websocket.TextMessage, b)
		wsMu.Unlock()
		return nil
	}
}

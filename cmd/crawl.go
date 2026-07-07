package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/asciimoo/hister/server/crawler"
	"github.com/asciimoo/hister/server/model"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "Manage persistent crawl jobs",
	Long:  "Manage persistent crawl jobs",
}

var crawlListCmd = &cobra.Command{
	Use:   "list",
	Short: "List persistent crawl jobs",
	Long:  "Display all persistent crawl jobs with their status and URL counts",
	Args:  cobra.NoArgs,
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
	},
	Run: func(cmd *cobra.Command, args []string) {
		jobs, err := model.ListCrawlJobs()
		if err != nil {
			exit(1, "Failed to list crawl jobs: "+err.Error())
		}
		if len(jobs) == 0 {
			fmt.Println("No crawl jobs found.")
			return
		}
		for _, j := range jobs {
			stats, err := model.GetCrawlJobStats(j.ID)
			if err != nil {
				log.Warn().Err(err).Str("job_id", j.ID).Msg("failed to get job stats")
			}
			fmt.Printf("%s  %-12s  %s\n",
				cliInfoStyle.Render(j.ID),
				j.Status,
				j.StartURL,
			)
			fmt.Printf("  pending: %d  done: %d  failed: %d  skipped: %d  created: %s\n",
				stats.Pending, stats.Done, stats.Failed, stats.Skipped,
				j.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}
	},
}

var crawlShowCmd = &cobra.Command{
	Use:   "show JOB_ID",
	Short: "Show detailed persistent crawl job state",
	Long:  "Display detailed information about a persistent crawl job and its queued URL state",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
	},
	Run: func(cmd *cobra.Command, args []string) {
		showCrawlJob(args[0])
	},
}

var crawlErrorsCmd = &cobra.Command{
	Use:   "errors JOB_ID",
	Short: "List failed crawl URLs",
	Long:  "List failed crawl URL error codes and URLs for a persistent crawl job",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
	},
	Run: func(cmd *cobra.Command, args []string) {
		showCrawlJobErrors(args[0])
	},
}

var crawlQueueCmd = &cobra.Command{
	Use:   "queue JOB_ID",
	Short: "List crawl queue URLs",
	Long:  "List crawl URL status, depth, and URL rows for a persistent crawl job",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
	},
	Run: func(cmd *cobra.Command, args []string) {
		showCrawlJobQueue(args[0])
	},
}

var crawlDeleteCmd = &cobra.Command{
	Use:   "delete JOB_ID",
	Short: "Delete a persistent crawl job",
	Long:  "Delete a crawl job and all its associated URL tracking data",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
	},
	Run: func(cmd *cobra.Command, args []string) {
		jobID := args[0]
		if err := model.DeleteCrawlJob(jobID); err != nil {
			exit(1, "Failed to delete crawl job: "+err.Error())
		}
		fmt.Println(cliSuccessStyle.Render("✓") + " Crawl job deleted: " + cliInfoStyle.Render(jobID))
	},
}

func showCrawlJob(jobID string) {
	job := loadCrawlJob(jobID)

	stats, err := model.GetCrawlJobStats(job.ID)
	if err != nil {
		exit(1, "Failed to load crawl job stats: "+err.Error())
	}

	fmt.Println(cliBoldStyle.Render("CRAWL JOB"))
	fmt.Printf("id: %s\n", cliInfoStyle.Render(job.ID))
	fmt.Printf("status: %s\n", job.Status)
	fmt.Printf("start_url: %s\n", job.StartURL)
	fmt.Printf("label: %s\n", job.Label)
	fmt.Printf("created: %s\n", job.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("updated: %s\n", job.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	fmt.Println(cliBoldStyle.Render("STATE"))
	fmt.Printf("pending: %d\n", stats.Pending)
	fmt.Printf("in_progress: %d\n", stats.InProgress)
	fmt.Printf("done: %d\n", stats.Done)
	fmt.Printf("failed: %d\n", stats.Failed)
	fmt.Printf("skipped: %d\n", stats.Skipped)
	fmt.Println()

	fmt.Println(cliBoldStyle.Render("RULES"))
	rules, err := crawler.UnmarshalValidatorRules(job.ValidatorRules)
	if err != nil {
		fmt.Println(job.ValidatorRules)
		log.Warn().Err(err).Str("job_id", job.ID).Msg("failed to restore crawl job rules")
	} else {
		rulesJSON, err := json.MarshalIndent(rules, "", "  ")
		if err != nil {
			exit(1, "Failed to format crawl job rules: "+err.Error())
		}
		fmt.Println(string(rulesJSON))
	}
}

func showCrawlJobQueue(jobID string) {
	job := loadCrawlJob(jobID)
	if err := model.ForEachCrawlURL(job.ID, func(status string, depth int, rawURL string) error {
		fmt.Printf("%s\t%d\t%s\n", status, depth, rawURL)
		return nil
	}); err != nil {
		exit(1, "Failed to load crawl job queue: "+err.Error())
	}
}

func showCrawlJobErrors(jobID string) {
	job := loadCrawlJob(jobID)
	if err := model.ForEachFailedCrawlURL(job.ID, func(errorCode int, rawURL string) error {
		fmt.Printf("%d\t%s\n", errorCode, rawURL)
		return nil
	}); err != nil {
		exit(1, "Failed to load crawl job errors: "+err.Error())
	}
}

func loadCrawlJob(jobID string) *model.CrawlJob {
	job, err := model.GetCrawlJob(jobID)
	if err != nil {
		exit(1, "Failed to load crawl job: "+err.Error())
	}
	if job == nil {
		exit(1, "Crawl job not found: "+jobID)
	}
	return job
}

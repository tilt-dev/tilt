package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type insightsCmd struct {
	output   string // json, table, summary
	since    string // duration string like "24h" or "7d"
	resource string // filter by resource
	limit    int    // limit results
}

func (c *insightsCmd) name() model.TiltSubcommand { return "insights" }

func (c *insightsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights [resource]",
		Short: "View build insights and analytics",
		Long: `Display build performance insights and analytics from the running Tilt instance.

This command shows:
- Build success/failure rates
- Average build times per resource
- Performance recommendations
- Build history and trends

Examples:
  # View summary of build insights
  tilt insights

  # View insights for a specific resource
  tilt insights frontend

  # View detailed JSON output
  tilt insights --output json

  # View insights for the last 7 days
  tilt insights --since 168h

  # View recent builds
  tilt insights --output builds --limit 20
`,
	}

	cmd.Flags().StringVarP(&c.output, "output", "o", "summary", "Output format: summary, table, json, builds")
	cmd.Flags().StringVar(&c.since, "since", "24h", "Show insights since this duration (e.g., 24h, 168h, 720h)")
	cmd.Flags().StringVarP(&c.resource, "resource", "r", "", "Filter by resource name")
	cmd.Flags().IntVarP(&c.limit, "limit", "l", 20, "Limit number of results for builds output")

	addConnectServerFlags(cmd)
	return cmd
}

func (c *insightsCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.insights", map[string]string{"output": c.output})
	defer a.Flush(time.Second)

	// If a resource is provided as argument, use it
	if len(args) > 0 {
		c.resource = args[0]
	}

	// Build the URL
	host := provideWebHost()
	port := provideWebPort()
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	switch c.output {
	case "json":
		return c.fetchAndPrintJSON(baseURL)
	case "builds":
		return c.fetchAndPrintBuilds(baseURL)
	case "table":
		return c.fetchAndPrintTable(baseURL)
	default:
		return c.fetchAndPrintSummary(baseURL)
	}
}

func (c *insightsCmd) fetchAndPrintSummary(baseURL string) error {
	url := fmt.Sprintf("%s/api/insights?since=%s", baseURL, c.since)
	if c.resource != "" {
		url = fmt.Sprintf("%s&resource=%s", url, c.resource)
	}

	insights, err := c.fetchInsights(url)
	if err != nil {
		return err
	}

	c.printSummary(insights)
	return nil
}

func (c *insightsCmd) fetchAndPrintJSON(baseURL string) error {
	url := fmt.Sprintf("%s/api/insights?since=%s", baseURL, c.since)
	if c.resource != "" {
		url = fmt.Sprintf("%s&resource=%s", url, c.resource)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Tilt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	return err
}

func (c *insightsCmd) fetchAndPrintBuilds(baseURL string) error {
	url := fmt.Sprintf("%s/api/insights/builds?limit=%d&since=%s", baseURL, c.limit, c.since)
	if c.resource != "" {
		url = fmt.Sprintf("%s&resource=%s", url, c.resource)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Tilt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	var builds []model.BuildMetric
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	c.printBuilds(builds)
	return nil
}

func (c *insightsCmd) fetchAndPrintTable(baseURL string) error {
	url := fmt.Sprintf("%s/api/insights?since=%s", baseURL, c.since)
	if c.resource != "" {
		url = fmt.Sprintf("%s&resource=%s", url, c.resource)
	}

	insights, err := c.fetchInsights(url)
	if err != nil {
		return err
	}

	c.printTable(insights)
	return nil
}

func (c *insightsCmd) fetchInsights(url string) (*model.BuildInsights, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Tilt: %w\nIs Tilt running? Try 'tilt up' first.", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("build insights not available - this may be a version mismatch")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	var insights model.BuildInsights
	if err := json.NewDecoder(resp.Body).Decode(&insights); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &insights, nil
}

func (c *insightsCmd) printSummary(insights *model.BuildInsights) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("═══════════════════════════════════════════════════════════")
	bold.Println("                    BUILD INSIGHTS DASHBOARD                ")
	bold.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Session Summary
	s := insights.Session
	bold.Println("📊 Session Summary")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf("   Total Builds:       %d\n", s.TotalBuilds)

	successRate := float64(0)
	if s.TotalBuilds > 0 {
		successRate = float64(s.SuccessfulBuilds) / float64(s.TotalBuilds) * 100
	}

	if successRate >= 90 {
		fmt.Printf("   Success Rate:       ")
		green.Printf("%.1f%%\n", successRate)
	} else if successRate >= 70 {
		fmt.Printf("   Success Rate:       ")
		yellow.Printf("%.1f%%\n", successRate)
	} else {
		fmt.Printf("   Success Rate:       ")
		red.Printf("%.1f%%\n", successRate)
	}

	fmt.Printf("   Successful:         ")
	green.Printf("%d\n", s.SuccessfulBuilds)
	fmt.Printf("   Failed:             ")
	if s.FailedBuilds > 0 {
		red.Printf("%d\n", s.FailedBuilds)
	} else {
		fmt.Println("0")
	}

	fmt.Printf("   Avg Duration:       %s\n", formatDuration(s.AverageDurationMs))
	fmt.Printf("   Total Duration:     %s\n", formatDuration(s.TotalDurationMs))
	fmt.Printf("   Live Updates:       %d\n", s.LiveUpdateCount)
	fmt.Printf("   Full Rebuilds:      %d\n", s.FullRebuildCount)
	fmt.Printf("   Resources:          %d\n", s.ResourceCount)

	if s.CurrentStreak > 0 {
		fmt.Printf("   Current Streak:     ")
		if s.StreakType == "success" {
			green.Printf("%d successful\n", s.CurrentStreak)
		} else {
			red.Printf("%d failed\n", s.CurrentStreak)
		}
	}
	fmt.Println()

	// Top Resources by Build Time
	if len(insights.Resources) > 0 {
		bold.Println("🏗️  Resources by Build Count")
		fmt.Println("───────────────────────────────────────────────────────────")

		// Sort by total builds
		sorted := make([]model.ResourceStats, len(insights.Resources))
		copy(sorted, insights.Resources)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TotalBuilds > sorted[j].TotalBuilds
		})

		// Show top 5
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}

		for i := 0; i < limit; i++ {
			r := sorted[i]
			status := "✓"
			statusColor := green
			if r.SuccessRate < 90 {
				status = "⚠"
				statusColor = yellow
			}
			if r.SuccessRate < 70 {
				status = "✗"
				statusColor = red
			}

			fmt.Printf("   ")
			statusColor.Printf("%s ", status)
			cyan.Printf("%-20s", truncate(string(r.ManifestName), 20))
			fmt.Printf(" %3d builds  ", r.TotalBuilds)
			fmt.Printf("avg %s  ", formatDuration(r.AverageDurationMs))
			statusColor.Printf("%.0f%% success\n", r.SuccessRate)
		}
		fmt.Println()
	}

	// Slowest Builds
	if len(insights.SlowestBuilds) > 0 {
		bold.Println("🐢 Slowest Builds")
		fmt.Println("───────────────────────────────────────────────────────────")

		limit := 5
		if len(insights.SlowestBuilds) < limit {
			limit = len(insights.SlowestBuilds)
		}

		for i := 0; i < limit; i++ {
			b := insights.SlowestBuilds[i]
			fmt.Printf("   ")
			yellow.Printf("%-20s", truncate(string(b.ManifestName), 20))
			fmt.Printf(" %s  ", formatDuration(b.DurationMs))
			fmt.Printf("%s\n", b.StartTime.Format("15:04:05"))
		}
		fmt.Println()
	}

	// Recommendations
	if len(insights.Recommendations) > 0 {
		bold.Println("💡 Recommendations")
		fmt.Println("───────────────────────────────────────────────────────────")

		limit := 3
		if len(insights.Recommendations) < limit {
			limit = len(insights.Recommendations)
		}

		for i := 0; i < limit; i++ {
			rec := insights.Recommendations[i]
			priority := "  "
			switch rec.Priority {
			case model.RecommendationPriorityHigh:
				priority = "❗"
			case model.RecommendationPriorityMedium:
				priority = "⚠️ "
			case model.RecommendationPriorityLow:
				priority = "ℹ️ "
			}

			fmt.Printf("   %s ", priority)
			bold.Printf("%s\n", rec.Title)
			fmt.Printf("      %s\n", rec.Description)
			if rec.PotentialSavingsMs > 0 {
				green.Printf("      Potential savings: %s per build\n", formatDuration(rec.PotentialSavingsMs))
			}
			fmt.Println()
		}
	}

	bold.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Generated at: %s\n", insights.GeneratedAt.Format(time.RFC3339))
	fmt.Println()
}

func (c *insightsCmd) printTable(insights *model.BuildInsights) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "RESOURCE\tBUILDS\tSUCCESS\tAVG TIME\tP95 TIME\tLIVE UPD\tSTATUS")
	fmt.Fprintln(w, "--------\t------\t-------\t--------\t--------\t--------\t------")

	for _, r := range insights.Resources {
		status := "OK"
		if r.SuccessRate < 90 {
			status = "WARN"
		}
		if r.SuccessRate < 70 {
			status = "FAIL"
		}

		fmt.Fprintf(w, "%s\t%d\t%.0f%%\t%s\t%s\t%d\t%s\n",
			truncate(string(r.ManifestName), 20),
			r.TotalBuilds,
			r.SuccessRate,
			formatDuration(r.AverageDurationMs),
			formatDuration(r.P95DurationMs),
			r.LiveUpdateCount,
			status,
		)
	}

	w.Flush()
}

func (c *insightsCmd) printBuilds(builds []model.BuildMetric) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "TIME\tRESOURCE\tDURATION\tTYPE\tSTATUS")
	fmt.Fprintln(w, "----\t--------\t--------\t----\t------")

	for _, b := range builds {
		buildType := "rebuild"
		if b.LiveUpdate {
			buildType = "live-upd"
		}

		status := "OK"
		if !b.Success {
			status = "FAIL"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			b.StartTime.Format("15:04:05"),
			truncate(string(b.ManifestName), 20),
			formatDuration(b.DurationMs),
			buildType,
			status,
		)
	}

	w.Flush()
}

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s + strings.Repeat(" ", maxLen-len(s))
	}
	return s[:maxLen-1] + "…"
}

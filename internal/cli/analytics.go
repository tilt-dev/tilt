package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const tiltAppName = "tilt"

var analyticsService analytics.Analytics

func initAnalytics(rootCmd *cobra.Command) error {
	var analyticsCmd *cobra.Command
	var err error

	analyticsService, analyticsCmd, err = analytics.Init(tiltAppName)
	if err != nil {
		return err
	}

	status, err := analytics.OptStatus()
	if err != nil {
		return err
	}

	if status == analytics.OptDefault {
		_, err := fmt.Fprintf(os.Stderr, "Send anonymized usage data to Windmill [y/n]? ")
		if err != nil {
			return err
		}

		buf := bufio.NewReader(os.Stdin)
		c, _, _ := buf.ReadRune()
		if c == rune(0) || c == '\n' || c == 'y' || c == 'Y' {
			err = analytics.SetOpt(analytics.OptIn)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(os.Stderr, "Thanks! Setting 'tilt analytics opt in'")
			if err != nil {
				return err
			}
		} else {
			err = analytics.SetOpt(analytics.OptOut)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(os.Stderr, "Thanks! Setting 'tilt analytics opt out'")
			if err != nil {
				return err
			}
		}

		_, err = fmt.Fprintln(os.Stderr, "You set can update your privacy preferences later with 'tilt analytics'")
		if err != nil {
			return err
		}
	}

	rootCmd.AddCommand(analyticsCmd)
	return nil
}

func provideAnalytics() (analytics.Analytics, error) {
	if analyticsService == nil {
		return nil, fmt.Errorf("internal error: no available analytics service")
	}
	return analyticsService, nil
}

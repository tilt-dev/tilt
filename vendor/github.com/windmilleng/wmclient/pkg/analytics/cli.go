package analytics

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/windmilleng/wmclient/pkg/dirs"
)

func analyticsStatus(_ *cobra.Command, args []string) error {
	choice, err := OptStatus()
	if err != nil {
		return err
	}

	fmt.Printf("analytics status : %s\n", choice)
	fmt.Printf("analytics user id: %s\n", getUserID())

	return nil
}

func analyticsOpt(_ *cobra.Command, args []string) (outerErr error) {
	defer func() {
		if outerErr == nil {
			return
		}
		fmt.Printf("choice can be one of {%s, %s}\n", OptIn, OptOut)
	}()
	if len(args) == 0 {
		fmt.Printf("choice can be one of {%s, %s}\n", OptIn, OptOut)
		return fmt.Errorf("no choice given; pass it as first arg: <tool> analytics opt <choice>")
	}
	choiceStr := args[0]
	_, err := SetOptStr(choiceStr)
	if err != nil {
		return err
	}
	d, err := dirs.UseWindmillDir()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote user collection strategy %q to file %v\n", choiceStr, filepath.Join(d.Root(), choiceFile))
	return nil
}

const choiceFile = "analytics/user/choice.txt"

func NewCommand() *cobra.Command {
	analytics := &cobra.Command{
		Use:   "analytics",
		Short: "info and status about windmill analytics",
		RunE:  analyticsStatus,
	}

	opt := &cobra.Command{
		Use:   "opt",
		Short: "opt-in or -out to windmill analytics collection/upload",
		RunE:  analyticsOpt,
	}
	analytics.AddCommand(opt)
	return analytics
}

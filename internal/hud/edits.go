package hud

import (
	"fmt"
	"path"
	"strings"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/rty"
)

type EditStatusLineComponent struct {
	bs buildStatus
}

var _ rty.Component = &EditStatusLineComponent{}

func NewEditStatusLine(buildStatus buildStatus) rty.Component {
	return &EditStatusLineComponent{
		bs: buildStatus,
	}
}

func (esl *EditStatusLineComponent) Size(availWidth, availHeight int) (int, int, error) {
	return availWidth, 1, nil
}

func (esl *EditStatusLineComponent) makeEditsComponents(width int) (rty.Component, rty.Component) {
	var filenames, filenamesEtAl rty.Component

	// fit as many filenames as possible into `width`
	if len(esl.bs.edits) > 0 {
		// technically this might be a multi-digit number and could lose digits as we make more filenames explicit,
		// but close enough
		spaceForFilenames := width - len(fmt.Sprintf(" (+%d more)", len(esl.bs.edits)))

		edits := esl.bs.edits
		s := ""
		for len(edits) > 0 {
			next := path.Base(edits[0])
			if s != "" {
				next = " " + next
			}
			if len(next) <= spaceForFilenames {
				spaceForFilenames -= len(next)
				s += next
				edits = edits[1:]
			} else {
				break
			}
		}

		filenames = rty.TextString(s)

		if len(edits) > 0 {
			if len(s) == 0 {
				filenamesEtAl = rty.TextString(fmt.Sprintf("%d files", len(edits)))
			} else {
				filenamesEtAl = rty.TextString(fmt.Sprintf(" (+%d more)", len(edits)))
			}
		}
	}
	return filenames, filenamesEtAl
}

func (esl *EditStatusLineComponent) Render(w rty.Writer, width, height int) error {
	offset := 0
	allocated := 0
	sb := rty.NewStringBuilder()
	indent := strings.Repeat(" ", 8)

	sb.Fg(esl.bs.statusColor).Textf("%s●", indent)
	sb.Fg(cLightText).Text(" TILT: ")
	sb.Fg(tcell.ColorDefault).Text(esl.bs.status)
	sb.Fg(cLightText).Text(esl.bs.editsPrefix)

	lhs := sb.Build()

	sb = rty.NewStringBuilder()
	sb.Text(" ").Fg(cLightText).Text("DURATION ")
	sb.Fg(tcell.ColorDefault).Textf("%s           ", esl.bs.duration)
	duration := sb.Build()

	lhsW, _, err := lhs.Size(width, 1)
	if err != nil {
		return err
	}
	allocated += lhsW

	durationW, _, err := duration.Size(width, 1)
	if err != nil {
		return err
	}
	allocated += durationW

	filenames, filenamesEtAl := esl.makeEditsComponents(width - allocated)
	var filenamesW, filenamesEtAlW int
	if filenamesEtAl != nil {
		filenamesEtAlW, _, err = filenamesEtAl.Size(width, 1)
		if err != nil {
			return err
		}
		allocated += filenamesEtAlW
	}

	if filenames != nil && allocated < width {
		filenamesW, _, err = filenames.Size(width-allocated, 1)
		if err != nil {
			return err
		}
	}

	{
		w, err := w.Divide(0, 0, width, 1)
		if err != nil {
			return err
		}
		w.RenderChild(lhs)
		offset += lhsW
	}

	if filenames != nil && filenamesW > 0 {
		w, err := w.Divide(offset, 0, filenamesW, 1)
		if err != nil {
			return err
		}
		w.RenderChild(filenames)
		offset += filenamesW
	}

	if filenamesEtAl != nil {
		w, err := w.Divide(offset, 0, filenamesEtAlW, 1)
		if err != nil {
			return err
		}
		w.RenderChild(filenamesEtAl)
	}

	{
		w, err := w.Divide(width-durationW, 0, durationW, 1)
		if err != nil {
			return err
		}
		w.RenderChild(duration)
	}

	return nil
}

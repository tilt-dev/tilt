package analysis

import (
	"context"

	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

type Analyzer struct {
	builtins *Builtins
	context  context.Context
	logger   *zap.Logger
}

type AnalyzerOption func(*Analyzer) error

func NewAnalyzer(ctx context.Context, opts ...AnalyzerOption) (*Analyzer, error) {
	analyzer := Analyzer{
		context:  ctx,
		builtins: NewBuiltins(),
	}
	logger := protocol.LoggerFromContext(ctx)
	logger = logger.Named("analyzer")
	analyzer.logger = logger

	for _, opt := range opts {
		err := opt(&analyzer)
		if err != nil {
			return &analyzer, err
		}
	}

	if len(analyzer.builtins.Functions) != 0 {
		logger.Debug("registered built-in functions", zap.Int("count", len(analyzer.builtins.Functions)))
	}
	if len(analyzer.builtins.Symbols) != 0 {
		logger.Debug("registered built-in symbols", zap.Int("count", len(analyzer.builtins.Symbols)))
	}

	return &analyzer, nil
}

package store

import "context"

type Reducer func(ctx context.Context, engineState *EngineState, action Action)

var EmptyReducer = Reducer(func(ctx context.Context, s *EngineState, action Action) {})

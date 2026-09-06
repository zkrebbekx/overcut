//go:build js && wasm

// Command overcut-wasm exposes the engine to a browser. Every function
// takes and returns JSON strings, so the JavaScript side stays thin.
//
//	overcut.load(seasonJSON)            -> "" or an error message
//	overcut.season()                    -> SeasonView
//	overcut.rules()                     -> rules.Config
//	overcut.project(inputJSON)          -> ProjectionView
//	overcut.optimize(inputJSON)         -> OptimizeView
//	overcut.prices()                    -> PricesView
//	overcut.backtest(sims)              -> backtest.Report
//	overcut.hindsight(round, top)       -> HindsightView
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/internal/engine"
	"github.com/zkrebbekx/overcut/rules"
)

var eng *engine.Engine

// reply encodes a result or an error as the JSON the worker expects.
func reply(v any, err error) string {
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	return string(b)
}

func main() {
	api := js.Global().Get("Object").New()

	api.Set("load", js.FuncOf(func(this js.Value, args []js.Value) any {
		var d dataset.Data
		if err := json.Unmarshal([]byte(args[0].String()), &d); err != nil {
			return err.Error()
		}
		eng = engine.New(d, rules.Default())
		return ""
	}))
	api.Set("season", js.FuncOf(func(this js.Value, args []js.Value) any {
		return reply(eng.Season(), nil)
	}))
	api.Set("rules", js.FuncOf(func(this js.Value, args []js.Value) any {
		return reply(eng.Rules(), nil)
	}))
	api.Set("project", js.FuncOf(func(this js.Value, args []js.Value) any {
		var in engine.ProjectInput
		if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
			return reply(nil, err)
		}
		return reply(eng.Project(in))
	}))
	api.Set("optimize", js.FuncOf(func(this js.Value, args []js.Value) any {
		var in engine.OptimizeInput
		if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
			return reply(nil, err)
		}
		return reply(eng.Optimize(in))
	}))
	api.Set("prices", js.FuncOf(func(this js.Value, args []js.Value) any {
		return reply(eng.Prices(), nil)
	}))
	api.Set("backtest", js.FuncOf(func(this js.Value, args []js.Value) any {
		return reply(eng.Backtest(args[0].Int()), nil)
	}))
	api.Set("hindsight", js.FuncOf(func(this js.Value, args []js.Value) any {
		return reply(eng.Hindsight(args[0].Int(), args[1].Int()))
	}))
	api.Set("review", js.FuncOf(func(this js.Value, args []js.Value) any {
		var in engine.ReviewInput
		if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
			return reply(nil, err)
		}
		return reply(eng.Review(in))
	}))

	js.Global().Set("overcut", api)
	select {}
}

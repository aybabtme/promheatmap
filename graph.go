package main

import (
	"image/color"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gonum/plot"
	"github.com/gonum/plot/plotter"
	"github.com/gonum/plot/vg"
	"github.com/gonum/plot/vg/draw"
	"github.com/prometheus/common/model"
)

func plotScatter(streams []*model.SampleStream, min model.SampleValue, unitsTicker func(plot.Ticker) plot.Ticker, title, filename string) error {

	p, err := plot.New()
	if err != nil {
		return err
	}

	p.Title.Text = title
	p.Y.Label.Text = "log10"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = unitsTicker(plot.LogTicks{})
	p.X.Label.Text = "Date"
	p.X.Tick.Marker = readableTime(time.Second, p.X.Tick.Marker)

	p.Add(plotter.NewGrid())

	scatter, err := plotter.NewScatter(func() plotter.XYs {
		var xys plotter.XYs
		for _, stream := range streams {
			for _, ev := range stream.Values {

				x := ev.Timestamp.Unix()
				if ev.Value < min {
					ev.Value = min
				}
				xys = append(xys, struct{ X, Y float64 }{
					X: float64(x), Y: float64(ev.Value),
				})
			}
		}
		return xys
	}())
	if err != nil {
		return err
	}
	scatter.Color = color.RGBA{166, 189, 219, 255}
	scatter.GlyphStyle.Shape = draw.PlusGlyph{}
	scatter.GlyphStyle.Radius = vg.Points(1)
	p.Add(scatter)

	width := 16 * vg.Inch
	height := 12 * vg.Inch
	return p.Save(width, height, filename)
}

type ticker func(float64, float64) []plot.Tick

func (t ticker) Ticks(min, max float64) []plot.Tick { return t(min, max) }

func readableDuration(marker plot.Ticker) plot.Ticker {
	return ticker(func(min, max float64) []plot.Tick {
		var out []plot.Tick
		for _, t := range marker.Ticks(min, max) {
			t.Label = time.Duration(t.Value).String()
			out = append(out, t)
		}
		return out
	})
}

func readableBytes(marker plot.Ticker) plot.Ticker {
	return ticker(func(min, max float64) []plot.Tick {
		var out []plot.Tick
		for _, t := range marker.Ticks(min, max) {

			t.Label = humanize.IBytes(uint64(t.Value))
			out = append(out, t)
		}
		return out
	})
}

func readableTime(truncate time.Duration, marker plot.Ticker) plot.Ticker {
	return ticker(func(min, max float64) []plot.Tick {
		var out []plot.Tick
		for _, t := range marker.Ticks(min, max) {
			t.Label = time.Unix(int64(t.Value), 0).Truncate(truncate).Format(time.RFC3339)
			out = append(out, t)
		}
		return out
	})
}

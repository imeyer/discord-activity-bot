package charts

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"sort"
	"time"

	"github.com/imeyer/discord-activity-bot/db"
	"github.com/imeyer/discord-activity-bot/internal/pkg"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gonum.org/v1/gonum/interp"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// GenerateChannelActivityImage creates a line chart image of channel activity using Gonum Plot
func GenerateChannelActivityImage(ctx context.Context, activity []db.GetChannelActivityTimelineRow, usernames map[string]string) ([]byte, error) {
	ctx, span := pkg.StartSpan(ctx, "charts.GenerateChannelActivityImage",
		attribute.Int("activity_points", len(activity)),
		attribute.Int("unique_users", len(usernames)),
	)
	defer span.End()

	start := time.Now()

	// Create time slots for the last 24 hours (24 hourly intervals)
	// Align with database hourly boundaries
	now := time.Now().UTC()
	roundedNow := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(),
		0, 0, 0, time.UTC)
	startTime := roundedNow.Add(-24 * time.Hour)

	// Get unique users
	uniqueUsers := make(map[string]bool)
	for _, point := range activity {
		uniqueUsers[point.UserID] = true
	}

	// Create interval data map
	intervalData := make(map[time.Time]map[string]float64)
	for _, point := range activity {
		if !point.IntervalStart.Valid {
			continue
		}
		intervalStart := point.IntervalStart.Time
		if intervalData[intervalStart] == nil {
			intervalData[intervalStart] = make(map[string]float64)
		}
		intervalData[intervalStart][point.UserID] = float64(point.MessageCount)
	}

	// Generate exactly 24 time slots (one per hour)
	var timeSlots []time.Time
	for interval := startTime; interval.Before(roundedNow); interval = interval.Add(1 * time.Hour) {
		timeSlots = append(timeSlots, interval)
	}

	// Set a clean sans-serif font (gonum should use system fallback)
	plot.DefaultFont.Typeface = "Helvetica"

	// Create new plot with custom styling
	p := plot.New()
	p.Title.Text = "Channel Activity Timeline (Last 24 Hours)"
	p.X.Label.Text = "Time"
	p.Y.Label.Text = "Messages"

	// Apply Discord dark theme with readable text
	darkGray := color.RGBA{R: 54, G: 57, B: 63, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	p.BackgroundColor = darkGray

	// Make all text white with better font sizes
	p.Title.TextStyle.Color = white
	p.Title.TextStyle.Font.Size = vg.Points(24)
	p.X.Label.TextStyle.Color = white
	p.X.Label.TextStyle.Font.Size = vg.Points(14)
	p.Y.Label.TextStyle.Color = white
	p.Y.Label.TextStyle.Font.Size = vg.Points(14)
	p.X.Tick.Label.Color = white
	p.X.Tick.Label.Font.Size = vg.Points(14)
	p.Y.Tick.Label.Color = white
	p.Y.Tick.Label.Font.Size = vg.Points(14)

	// Position legend with white text and larger font
	p.Legend.Top = true
	p.Legend.Left = true
	p.Legend.TextStyle.Color = white
	p.Legend.TextStyle.Font.Size = vg.Points(18)

	// Sort users for consistent ordering
	var sortedUsers []string
	for userID := range uniqueUsers {
		sortedUsers = append(sortedUsers, userID)
	}
	sort.Strings(sortedUsers)

	// Add series for each user with vibrant contrasting colors
	colors := []color.RGBA{
		{R: 114, G: 137, B: 218, A: 255}, // Discord Blurple
		{R: 87, G: 242, B: 135, A: 255},  // Bright Green
		{R: 255, G: 100, B: 100, A: 255}, // Bright Red
		{R: 255, G: 215, B: 0, A: 255},   // Gold
		{R: 255, G: 20, B: 147, A: 255},  // Deep Pink
		{R: 0, G: 191, B: 255, A: 255},   // Deep Sky Blue
		{R: 255, G: 140, B: 0, A: 255},   // Dark Orange
		{R: 138, G: 43, B: 226, A: 255},  // Blue Violet
	}

	for i, userID := range sortedUsers {
		username := usernames[userID]
		if username == "" {
			username = "Unknown"
		}

		// Create XY data points for this user
		pts := make(plotter.XYs, len(timeSlots))
		for j, timeSlot := range timeSlots {
			value := 0.0
			if intervalData[timeSlot] != nil && intervalData[timeSlot][userID] > 0 {
				value = intervalData[timeSlot][userID]
			}
			// Use actual time values for proper X-axis labeling
			pts[j].X = float64(timeSlot.Unix())
			pts[j].Y = value
		}

		// Create line plotter (try smooth interpolation if enough points)
		var line *plotter.Line
		var err error

		if len(pts) >= 4 { // Need at least 4 points for decent interpolation
			// Extract X and Y values for interpolation
			xs := make([]float64, len(pts))
			ys := make([]float64, len(pts))
			for j, pt := range pts {
				xs[j] = pt.X
				ys[j] = pt.Y
			}

			// Create cubic spline interpolator
			spline := interp.AkimaSpline{}
			err := spline.Fit(xs, ys)
			if err != nil {
				// Fallback to regular line if spline fails
				line, err = plotter.NewLine(pts)
			} else {
				// Generate more points for smooth curve (4x density)
				smoothPts := make(plotter.XYs, len(pts)*4)
				for j := 0; j < len(smoothPts); j++ {
					t := float64(j) / float64(len(smoothPts)-1)
					x := xs[0] + t*(xs[len(xs)-1]-xs[0])
					y := spline.Predict(x)
					smoothPts[j].X = x
					smoothPts[j].Y = y
				}

				// Create line with smoothed points
				line, err = plotter.NewLine(smoothPts)
			}
		} else {
			// Fallback to regular line for few points
			line, err = plotter.NewLine(pts)
		}

		if err != nil {
			pkg.RecordError(ctx, err, "failed_to_create_line")
			continue
		}

		// Set line style with Discord colors and smooth curves
		line.Color = colors[i%len(colors)]
		line.Width = vg.Points(3)
		line.Dashes = []vg.Length{} // Solid line

		// Add to plot
		p.Add(line)
		p.Legend.Add(username, line)
	}

	// Find the maximum Y value across all data points
	maxY := 0.0
	for _, point := range activity {
		if float64(point.MessageCount) > maxY {
			maxY = float64(point.MessageCount)
		}
	}

	// Set Y-axis range with ceiling of max + 20% (minimum +2 for small numbers)
	if maxY > 0 {
		padding := maxY * 0.2 // 20% padding
		if padding < 2 {
			padding = 2 // Minimum padding for small values
		}
		p.Y.Min = 0
		p.Y.Max = maxY + padding
	}

	// Set X-axis to show proper time range and labels
	p.X.Min = float64(startTime.Unix())
	p.X.Max = float64(roundedNow.Unix())
	p.X.Tick.Marker = plot.TimeTicks{Format: "15:04"}

	pkg.AddSpanEvent(ctx, "rendering_chart")

	// Create larger canvas with dark background padding
	canvasWidth := vg.Points(1000)
	canvasHeight := vg.Points(700)
	padding := vg.Points(25)

	img := vgimg.New(canvasWidth, canvasHeight)
	dc := draw.New(img)

	// Fill entire canvas with dark background
	dc.SetColor(darkGray)
	dc.Fill(vg.Path{
		{Type: vg.MoveComp, Pos: vg.Point{X: 0, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: canvasHeight}},
		{Type: vg.LineComp, Pos: vg.Point{X: 0, Y: canvasHeight}},
		{Type: vg.CloseComp},
	})

	// Create padded area for the plot
	plotArea := draw.Crop(dc, padding, -padding, padding, -padding)
	p.Draw(plotArea)

	// Convert to PNG bytes
	buffer := bytes.NewBuffer([]byte{})
	w := vgimg.PngCanvas{Canvas: img}

	_, err := w.WriteTo(buffer)
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_write_image")
		return nil, err
	}

	// Record metrics
	duration := time.Since(start)
	pkg.EnsureInitialized()
	pkg.ImageGenerationTimer.Record(ctx, duration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("chart_type", "channel_activity"),
			attribute.Int("data_points", len(activity)),
		),
	)

	pkg.AddSpanAttributes(ctx,
		attribute.Int("output_bytes", len(buffer.Bytes())),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	return buffer.Bytes(), nil
}

// GeneratePeakHoursHeatmap creates a heatmap image of peak hours
func GeneratePeakHoursHeatmap(ctx context.Context, data []db.GetChannelPeakHoursRow) ([]byte, error) {
	ctx, span := pkg.StartSpan(ctx, "charts.GeneratePeakHoursHeatmap",
		attribute.Int("data_points", len(data)),
	)
	defer span.End()

	start := time.Now()

	// Create map for easy lookup
	hourMap := make(map[int32]int64)
	maxCount := int64(0)
	for _, d := range data {
		hourMap[d.HourOfDay] = d.MessageCount
		if d.MessageCount > maxCount {
			maxCount = d.MessageCount
		}
	}

	// Create bar chart data
	pts := make(plotter.XYs, 24)
	colors := make([]color.RGBA, 24)

	for hour := 0; hour < 24; hour++ {
		count := hourMap[int32(hour)]
		pts[hour].X = float64(hour)
		pts[hour].Y = float64(count)
		
		// Calculate intensity and color
		intensity := float64(count) / float64(maxCount)
		colors[hour] = getHeatmapColorRGBA(intensity)
	}

	// Create plot
	p := plot.New()
	p.Title.Text = "Activity by Hour (Last 30 Days)"
	p.X.Label.Text = "Hour"
	p.Y.Label.Text = "Messages"

	// Apply Discord dark theme
	darkGray := color.RGBA{R: 54, G: 57, B: 63, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	p.BackgroundColor = darkGray
	p.Title.TextStyle.Color = white
	p.Title.TextStyle.Font.Size = vg.Points(16)
	p.X.Label.TextStyle.Color = white
	p.Y.Label.TextStyle.Color = white
	p.X.Tick.Label.Color = white
	p.Y.Tick.Label.Color = white

	// Create bars with gradient colors
	bars, err := plotter.NewBarChart(plotter.Values(extractYValues(pts)), vg.Points(35))
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_create_bars")
		return nil, err
	}

	// Note: gonum plot doesn't support per-bar colors easily, using average color
	bars.Color = color.RGBA{R: 114, G: 137, B: 218, A: 255} // Discord Blurple
	p.Add(bars)

	// Set X-axis to show hours
	p.X.Min = -0.5
	p.X.Max = 23.5

	pkg.AddSpanEvent(ctx, "rendering_chart")

	// Create canvas and render
	canvasWidth := vg.Points(1200)
	canvasHeight := vg.Points(600)
	padding := vg.Points(20)

	img := vgimg.New(canvasWidth, canvasHeight)
	dc := draw.New(img)

	// Fill background
	dc.SetColor(darkGray)
	dc.Fill(vg.Path{
		{Type: vg.MoveComp, Pos: vg.Point{X: 0, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: canvasHeight}},
		{Type: vg.LineComp, Pos: vg.Point{X: 0, Y: canvasHeight}},
		{Type: vg.CloseComp},
	})

	plotArea := draw.Crop(dc, padding, -padding, padding, -padding)
	p.Draw(plotArea)

	// Convert to PNG
	buffer := bytes.NewBuffer([]byte{})
	w := vgimg.PngCanvas{Canvas: img}
	_, err = w.WriteTo(buffer)
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_write_image")
		return nil, err
	}

	// Record metrics
	duration := time.Since(start)
	pkg.EnsureInitialized()
	pkg.ImageGenerationTimer.Record(ctx, duration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("chart_type", "peak_hours_heatmap"),
			attribute.Int("data_points", len(data)),
		),
	)

	pkg.AddSpanAttributes(ctx,
		attribute.Int("output_bytes", len(buffer.Bytes())),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	return buffer.Bytes(), nil
}

// GenerateRisingStarsChart creates a horizontal bar chart for rising stars
func GenerateRisingStarsChart(ctx context.Context, stars []db.GetRisingStarsRow, usernames map[string]string) ([]byte, error) {
	ctx, span := pkg.StartSpan(ctx, "charts.GenerateRisingStarsChart",
		attribute.Int("stars_count", len(stars)),
	)
	defer span.End()

	start := time.Now()
	// Limit to top 10
	limit := 10
	if len(stars) < limit {
		limit = len(stars)
	}

	// Create plot
	p := plot.New()
	p.Title.Text = "Rising Stars - This Week's Activity"
	p.X.Label.Text = "Messages This Week"
	p.Y.Label.Text = "Users"

	// Apply Discord dark theme
	darkGray := color.RGBA{R: 54, G: 57, B: 63, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	p.BackgroundColor = darkGray
	p.Title.TextStyle.Color = white
	p.Title.TextStyle.Font.Size = vg.Points(16)
	p.X.Label.TextStyle.Color = white
	p.Y.Label.TextStyle.Color = white
	p.X.Tick.Label.Color = white
	p.Y.Tick.Label.Color = white

	// Create horizontal bar data
	pts := make(plotter.XYs, limit)
	labels := make([]string, limit)

	for i := 0; i < limit; i++ {
		star := stars[i]
		username := usernames[star.UserID]
		if username == "" {
			username = "Unknown"
		}

		// Truncate long usernames
		if len(username) > 15 {
			username = username[:14] + "…"
		}

		pts[i].X = float64(star.ThisWeek)
		pts[i].Y = float64(limit - i - 1) // Reverse order for top-to-bottom
		labels[i] = fmt.Sprintf("%s (+%.0f%%)", username, star.GrowthRate)
	}

	// Create horizontal bars
	bars, err := plotter.NewBarChart(plotter.Values(extractXValues(pts)), vg.Points(40))
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_create_bars")
		return nil, err
	}
	bars.Horizontal = true
	bars.Color = color.RGBA{R: 87, G: 242, B: 135, A: 255} // Bright Green
	p.Add(bars)

	// Set custom Y-axis labels
	p.Y.Tick.Marker = plot.ConstantTicks(generateTicks(labels))
	p.Y.Min = -0.5
	p.Y.Max = float64(limit) - 0.5

	pkg.AddSpanEvent(ctx, "rendering_chart")

	// Create canvas and render
	canvasWidth := vg.Points(1200)
	canvasHeight := vg.Points(600)
	padding := vg.Points(20)

	img := vgimg.New(canvasWidth, canvasHeight)
	dc := draw.New(img)

	// Fill background
	dc.SetColor(darkGray)
	dc.Fill(vg.Path{
		{Type: vg.MoveComp, Pos: vg.Point{X: 0, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: canvasHeight}},
		{Type: vg.LineComp, Pos: vg.Point{X: 0, Y: canvasHeight}},
		{Type: vg.CloseComp},
	})

	plotArea := draw.Crop(dc, padding, -padding, padding, -padding)
	p.Draw(plotArea)

	// Convert to PNG
	buffer := bytes.NewBuffer([]byte{})
	w := vgimg.PngCanvas{Canvas: img}
	_, err = w.WriteTo(buffer)
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_write_image")
		return nil, err
	}

	// Record metrics
	duration := time.Since(start)
	pkg.EnsureInitialized()
	pkg.ImageGenerationTimer.Record(ctx, duration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("chart_type", "rising_stars"),
			attribute.Int("data_points", len(stars)),
		),
	)

	pkg.AddSpanAttributes(ctx,
		attribute.Int("output_bytes", len(buffer.Bytes())),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	return buffer.Bytes(), nil
}

// getHeatmapColorRGBA returns a color based on intensity (0.0 to 1.0)
func getHeatmapColorRGBA(intensity float64) color.RGBA {
	// Gradient from blue (cold) to red (hot)
	if intensity < 0.25 {
		// Blue to cyan
		return color.RGBA{R: 0, G: uint8(255 * intensity * 4), B: 255, A: 255}
	} else if intensity < 0.5 {
		// Cyan to green
		t := (intensity - 0.25) * 4
		return color.RGBA{R: 0, G: 255, B: uint8(255 * (1 - t)), A: 255}
	} else if intensity < 0.75 {
		// Green to yellow
		t := (intensity - 0.5) * 4
		return color.RGBA{R: uint8(255 * t), G: 255, B: 0, A: 255}
	} else {
		// Yellow to red
		t := (intensity - 0.75) * 4
		return color.RGBA{R: 255, G: uint8(255 * (1 - t)), B: 0, A: 255}
	}
}

// generateTicks creates tick marks for custom labels
func generateTicks(labels []string) []plot.Tick {
	ticks := make([]plot.Tick, len(labels))
	for i, label := range labels {
		ticks[i] = plot.Tick{
			Value: float64(len(labels) - i - 1),
			Label: label,
		}
	}
	return ticks
}

// GenerateServerActivityDonut creates a pie chart for server activity breakdown
func GenerateServerActivityDonut(ctx context.Context, summary db.GetServerActivitySummaryRow) ([]byte, error) {
	ctx, span := pkg.StartSpan(ctx, "charts.GenerateServerActivityDonut")
	defer span.End()

	start := time.Now()
	activePercent := float64(summary.ActiveUsersWeek) / float64(summary.TotalUsers) * 100
	inactivePercent := 100 - activePercent

	// Create labels for the chart
	labels := []string{
		fmt.Sprintf("Active (%.1f%%)", activePercent),
		fmt.Sprintf("Inactive (%.1f%%)", inactivePercent),
	}

	// Create plot
	p := plot.New()
	p.Title.Text = "Weekly User Engagement"
	p.HideAxes()

	// Apply Discord dark theme
	darkGray := color.RGBA{R: 54, G: 57, B: 63, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	p.BackgroundColor = darkGray
	p.Title.TextStyle.Color = white
	p.Title.TextStyle.Font.Size = vg.Points(16)

	// Create simple bar chart instead of pie (gonum doesn't have built-in pie charts)
	pts := make(plotter.XYs, 2)
	pts[0].X = 0
	pts[0].Y = activePercent
	pts[1].X = 1
	pts[1].Y = inactivePercent

	bars, err := plotter.NewBarChart(plotter.Values{activePercent, inactivePercent}, vg.Points(100))
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_create_bars")
		return nil, err
	}

	bars.Color = color.RGBA{R: 87, G: 242, B: 135, A: 255} // Green
	p.Add(bars)

	// Set custom X-axis labels
	p.X.Min = -0.5
	p.X.Max = 1.5
	p.X.Tick.Marker = plot.ConstantTicks([]plot.Tick{
		{Value: 0, Label: labels[0]},
		{Value: 1, Label: labels[1]},
	})

	pkg.AddSpanEvent(ctx, "rendering_chart")

	// Create canvas and render
	canvasWidth := vg.Points(800)
	canvasHeight := vg.Points(800)
	padding := vg.Points(20)

	img := vgimg.New(canvasWidth, canvasHeight)
	dc := draw.New(img)

	// Fill background
	dc.SetColor(darkGray)
	dc.Fill(vg.Path{
		{Type: vg.MoveComp, Pos: vg.Point{X: 0, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: 0}},
		{Type: vg.LineComp, Pos: vg.Point{X: canvasWidth, Y: canvasHeight}},
		{Type: vg.LineComp, Pos: vg.Point{X: 0, Y: canvasHeight}},
		{Type: vg.CloseComp},
	})

	plotArea := draw.Crop(dc, padding, -padding, padding, -padding)
	p.Draw(plotArea)

	// Convert to PNG
	buffer := bytes.NewBuffer([]byte{})
	w := vgimg.PngCanvas{Canvas: img}
	_, err = w.WriteTo(buffer)
	if err != nil {
		pkg.RecordError(ctx, err, "failed_to_write_image")
		return nil, err
	}

	// Record metrics
	duration := time.Since(start)
	pkg.EnsureInitialized()
	pkg.ImageGenerationTimer.Record(ctx, duration.Milliseconds(),
		metric.WithAttributes(
			attribute.String("chart_type", "server_activity_donut"),
		),
	)

	pkg.AddSpanAttributes(ctx,
		attribute.Int("output_bytes", len(buffer.Bytes())),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	return buffer.Bytes(), nil
}

// extractYValues extracts Y values from XY points for bar charts
func extractYValues(pts plotter.XYs) []float64 {
	values := make([]float64, len(pts))
	for i, pt := range pts {
		values[i] = pt.Y
	}
	return values
}

// extractXValues extracts X values from XY points for bar charts
func extractXValues(pts plotter.XYs) []float64 {
	values := make([]float64, len(pts))
	for i, pt := range pts {
		values[i] = pt.X
	}
	return values
}

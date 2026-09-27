package similarity

import "fmt"

// AnalyzeComposition returns deterministic, explicitly heuristic composition descriptors.
// It is not object detection, segmentation, or a learned vision model.
func AnalyzeComposition(data []byte) (map[string]any, error) {
	s, err := imageStats(data)
	if err != nil { return nil, fmt.Errorf("composition analysis: %w", err) }
	focalStrength := clamp01((1 - s.EdgeDensity) * (0.5 + 0.5*s.Variance))
	const boxSize = 0.5
	x0 := clamp01(s.CenterX - boxSize/2)
	y0 := clamp01(s.CenterY - boxSize/2)
	x1 := clamp01(x0 + boxSize)
	y1 := clamp01(y0 + boxSize)
	perspectiveHint := "standard"
	switch { case s.Aspect > 1.5: perspectiveHint = "landscape-aspect"; case s.Aspect < 0.75: perspectiveHint = "portrait-aspect" }
	return map[string]any{
		"analysis_mode": "deterministic-image-descriptors",
		"uncertainty": "Heuristic focal point and bounding box; no object detector or segmentation model is used.",
		"aspect_ratio": s.Aspect,
		"focal_points": []any{map[string]any{"x":s.CenterX,"y":s.CenterY,"strength":focalStrength}},
		"bounding_boxes": []any{map[string]any{"x_min":x0,"y_min":y0,"x_max":x1,"y_max":y1,"confidence":0.25}},
		"relative_positions": map[string]any{"luminance_center_x":s.CenterX,"luminance_center_y":s.CenterY},
		"horizon": s.CenterY,
		"camera_elevation": nil,
		"perspective": perspectiveHint,
		"hierarchy": []any{"luminance_center"},
		"negative_space": map[string]any{"edge_density":s.EdgeDensity,"estimated":true},
		"dominant_geometry": map[string]any{"aspect_ratio":s.Aspect,"edge_density":s.EdgeDensity},
		"object_scale": map[string]any{"heuristic_box_area":(x1-x0)*(y1-y0)},
		"light_direction": map[string]any{"estimated":false},
	}, nil
}

package brief

import "testing"

func TestInputValidate(t *testing.T) {
	aspect := "16:9"
	width := 1920
	height := 1080
	input := Input{
		Title: "Product hero",
		Goal: "Create a website hero image for the product page.",
		AspectRatio: &aspect,
		TargetWidth: &width,
		TargetHeight: &height,
		MustHave: []string{"product fully visible"},
		Avoid: []string{"hands"},
		SuccessCriteria: []string{"clear negative space for headline"},
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
}

func TestInputValidateRejectsInvalidAspectRatio(t *testing.T) {
	for _, value := range []string{"16", "0:9", "16:0", "sixteen:nine", "16/9"} {
		value := value
		t.Run(value, func(t *testing.T) {
			input := Input{Title: "x", Goal: "y", AspectRatio: &value}
			if err := input.Validate(); err == nil {
				t.Fatalf("expected invalid aspect ratio %q to fail", value)
			}
		})
	}
}

func TestInputValidateRejectsOversizedList(t *testing.T) {
	items := make([]string, 51)
	input := Input{Title: "x", Goal: "y", MustHave: items}
	if err := input.Validate(); err == nil {
		t.Fatal("expected oversized list to fail")
	}
}

func TestInputValidateRejectsBlankListItem(t *testing.T) {
	input := Input{Title: "x", Goal: "y", MustHave: []string{"ok", "   "}}
	if err := input.Validate(); err == nil {
		t.Fatal("expected blank list item to fail")
	}
}

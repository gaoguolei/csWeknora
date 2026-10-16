package sandbox

import "testing"

func TestIsStandardTemplateRecognizesProviderScopedName(t *testing.T) {
	for _, name := range []string{"weknora", "team/weknora", "project-b89e/WeKnora"} {
		if !isStandardTemplate(name) {
			t.Fatalf("expected %q to identify the standard template", name)
		}
	}
	if isStandardTemplate("weknora-custom") {
		t.Fatal("custom template is not the standard template")
	}
}

func TestClassifyWeKnoraTemplate(t *testing.T) {
	for _, tc := range []struct {
		name, image string
		want        bool
	}{
		{"weknora", "custom-image:v1", true},
		{"", DefaultDockerImage, true},
		{"", DefaultCubeTemplateImage, true},
		{"", "wechatopenai/weknora-sandbox:main-desktop-cube", false},
		{"weknora-desktop", DefaultDockerImage, false},
		{"custom", "other/image:v1", false},
	} {
		if got := classifyWeKnoraTemplate(tc.name, tc.image); got != tc.want {
			t.Errorf("classify %q %q = %v, want %v", tc.name, tc.image, got, tc.want)
		}
	}
}

package docker

import "testing"

func TestSelectImageTagsToRemove_IgnoreDeploymentCountsTowardKeepLimit(t *testing.T) {
	candidates := []removableImageTag{
		{Tag: "app:20260222010101", DeploymentID: "20260222010101", ImageID: "img-1"},
		{Tag: "app:20260222010102", DeploymentID: "20260222010102", ImageID: "img-2"},
		{Tag: "app:20260222010103", DeploymentID: "20260222010103", ImageID: "img-3"},
	}

	removals := selectImageTagsToRemove(candidates, map[string]struct{}{}, 2, "20260222010104")

	if len(removals) != 2 {
		t.Fatalf("len(removals) = %d, want 2", len(removals))
	}
	if removals[0].Tag != "app:20260222010102" {
		t.Fatalf("removals[0].Tag = %q, want %q", removals[0].Tag, "app:20260222010102")
	}
	if removals[1].Tag != "app:20260222010101" {
		t.Fatalf("removals[1].Tag = %q, want %q", removals[1].Tag, "app:20260222010101")
	}
}

func TestSelectImageTagsToRemove_KeepCurrentOnlyRemovesOlderTags(t *testing.T) {
	candidates := []removableImageTag{
		{Tag: "app:20260222010101", DeploymentID: "20260222010101", ImageID: "img-1"},
		{Tag: "app:20260222010102", DeploymentID: "20260222010102", ImageID: "img-2"},
	}

	removals := selectImageTagsToRemove(candidates, map[string]struct{}{}, 1, "20260222010103")

	if len(removals) != 2 {
		t.Fatalf("len(removals) = %d, want 2", len(removals))
	}
	if removals[0].Tag != "app:20260222010102" {
		t.Fatalf("removals[0].Tag = %q, want %q", removals[0].Tag, "app:20260222010102")
	}
	if removals[1].Tag != "app:20260222010101" {
		t.Fatalf("removals[1].Tag = %q, want %q", removals[1].Tag, "app:20260222010101")
	}
}

func TestSelectImageTagsToRemove_PreservesInUseImageWithoutKeptTag(t *testing.T) {
	candidates := []removableImageTag{
		{Tag: "app:20260222010101", DeploymentID: "20260222010101", ImageID: "img-1"},
	}

	removals := selectImageTagsToRemove(candidates, map[string]struct{}{"img-1": {}}, 0, "20260222010102")

	if len(removals) != 0 {
		t.Fatalf("len(removals) = %d, want 0", len(removals))
	}
}

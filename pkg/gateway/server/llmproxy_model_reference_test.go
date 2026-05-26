package server

import (
	"context"
	"testing"
	"time"

	types2 "github.com/obot-platform/obot/apiclient/types"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	storagescheme "github.com/obot-platform/obot/pkg/storage/scheme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// newModelFakeClient returns a fake client that supports the
// spec.manifest.modelProvider and spec.manifest.targetModel field selectors
// used by getModelByTargetForProvider.
func newModelFakeClient(objects ...kclient.Object) kclient.Client {
	return fake.NewClientBuilder().
		WithScheme(storagescheme.Scheme).
		WithIndex(&v1.Model{}, "spec.manifest.modelProvider", func(obj kclient.Object) []string {
			m := obj.(*v1.Model)
			if m.Spec.Manifest.ModelProvider == "" {
				return nil
			}
			return []string{m.Spec.Manifest.ModelProvider}
		}).
		WithIndex(&v1.Model{}, "spec.manifest.targetModel", func(obj kclient.Object) []string {
			m := obj.(*v1.Model)
			if m.Spec.Manifest.TargetModel == "" {
				return nil
			}
			return []string{m.Spec.Manifest.TargetModel}
		}).
		WithObjects(objects...).
		Build()
}

func TestGetModelFromReference_ReturnsNotFoundWhenNameAndTargetMiss(t *testing.T) {
	client := fake.NewClientBuilder().
		WithScheme(storagescheme.Scheme).
		Build()

	_, err := getModelFromReference(context.Background(), client, "default", "missing-model")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found error type, got %T: %v", err, err)
	}
}

func TestGetModelFromReference_ReturnsModelByResourceName(t *testing.T) {
	model := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openai-gpt-4.1-mini",
			Namespace: "default",
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			Name:        "manifest-name",
			TargetModel: "target-model-id",
			Active:      true,
		}},
	}

	client := fake.NewClientBuilder().
		WithScheme(storagescheme.Scheme).
		WithObjects(model).
		Build()

	got, err := getModelFromReference(context.Background(), client, "default", "openai-gpt-4.1-mini")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Name != "openai-gpt-4.1-mini" {
		t.Fatalf("expected openai-gpt-4.1-mini, got %q", got.Name)
	}
}

func TestGetModelFromReference_DoesNotFallbackToManifestNameOrTargetModel(t *testing.T) {
	model := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openai-gpt-4.1-mini",
			Namespace: "default",
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			Name:        "manifest-name",
			TargetModel: "target-model-id",
			Active:      true,
		}},
	}

	client := fake.NewClientBuilder().
		WithScheme(storagescheme.Scheme).
		WithObjects(model).
		Build()

	for _, ref := range []string{"manifest-name", "target-model-id"} {
		_, err := getModelFromReference(context.Background(), client, "default", ref)
		if err == nil {
			t.Fatalf("%s: expected error, got nil", ref)
		}

		if !apierrors.IsNotFound(err) {
			t.Fatalf("%s: expected not found error type, got %T: %v", ref, err, err)
		}
	}
}

func TestGetModelByTargetForProvider_FindsActiveModel(t *testing.T) {
	model := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "m1-openai-gpt-4o",
			Namespace: "default",
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			Name:          "GPT-4o",
			TargetModel:   "gpt-4o",
			ModelProvider: "openai-model-provider",
			Active:        true,
		}},
	}

	client := newModelFakeClient(model)

	got, err := getModelByTargetForProvider(context.Background(), client, "default", "openai-model-provider", "gpt-4o")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Name != "m1-openai-gpt-4o" {
		t.Fatalf("expected m1-openai-gpt-4o, got %q", got.Name)
	}
}

func TestGetModelByTargetForProvider_IgnoresWrongProvider(t *testing.T) {
	// Same TargetModel ("gpt-4o") under a different provider must not match.
	model := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "m1-other-gpt-4o",
			Namespace: "default",
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			TargetModel:   "gpt-4o",
			ModelProvider: "some-other-provider",
			Active:        true,
		}},
	}

	client := newModelFakeClient(model)

	_, err := getModelByTargetForProvider(context.Background(), client, "default", "openai-model-provider", "gpt-4o")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found error type, got %T: %v", err, err)
	}
}

func TestGetModelByTargetForProvider_IgnoresInactive(t *testing.T) {
	model := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "m1-openai-gpt-4o-inactive",
			Namespace: "default",
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			TargetModel:   "gpt-4o",
			ModelProvider: "openai-model-provider",
			Active:        false,
		}},
	}

	client := newModelFakeClient(model)

	_, err := getModelByTargetForProvider(context.Background(), client, "default", "openai-model-provider", "gpt-4o")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found error type, got %T: %v", err, err)
	}
}

func TestGetModelByTargetForProvider_NewestWinsOnDuplicate(t *testing.T) {
	// Defensive test for the theoretical case where two active v1.Model
	// entries share the same (provider, targetModel). Newest must win.
	older := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "m1-openai-gpt-4o-older",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			TargetModel:   "gpt-4o",
			ModelProvider: "openai-model-provider",
			Active:        true,
		}},
	}
	newer := &v1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "m1-openai-gpt-4o-newer",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Spec: v1.ModelSpec{Manifest: types2.ModelManifest{
			TargetModel:   "gpt-4o",
			ModelProvider: "openai-model-provider",
			Active:        true,
		}},
	}

	client := newModelFakeClient(older, newer)

	got, err := getModelByTargetForProvider(context.Background(), client, "default", "openai-model-provider", "gpt-4o")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Name != "m1-openai-gpt-4o-newer" {
		t.Fatalf("expected newer model to win, got %q", got.Name)
	}
}

func TestGetModelByTargetForProvider_NotFound(t *testing.T) {
	client := newModelFakeClient()

	_, err := getModelByTargetForProvider(context.Background(), client, "default", "openai-model-provider", "gpt-4o")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected not found error type, got %T: %v", err, err)
	}
}

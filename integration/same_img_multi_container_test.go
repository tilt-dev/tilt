//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSameImgMultiContainer(t *testing.T) {
	fmt.Println("~~ setting up fixture")
	f := newK8sFixture(t, "same_img_multi_container")
	defer f.TearDown()

	f.TiltWatch()
	fmt.Println("~~ tilt up-ing")

	// ForwardPort will fail if all the pods are not ready.
	//
	// We can't use the normal Tilt-managed forwards here because
	// Tilt doesn't setup forwards when --watch=false.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	fmt.Println("~~ waiting for pods ready")
	firstPods := f.WaitForAllPodsReady(ctx, "app=sameimg")
	fmt.Println("got pods:", firstPods)

	fmt.Println("~~ forwarding ports")
	f.ForwardPort("deployment/sameimg", "8100:8000") // container 1
	f.ForwardPort("deployment/sameimg", "8101:8001") // container 2

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	fmt.Println("~~ curling for c1")
	f.CurlUntil(ctx, "http://localhost:8100", "🍄 One-Up! 🍄")
	fmt.Println("~~ curling for c2")
	f.CurlUntil(ctx, "http://localhost:8101", "🍄 One-Up! 🍄")
	fmt.Println("~~ 🎉")

	f.ReplaceContents("main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "🍄 Two-Up! 🍄")
	fmt.Println("~~ curling for c2")
	f.CurlUntil(ctx, "http://localhost:8101", "🍄 Two-Up! 🍄")

	fmt.Println("~~ zzzz")
	time.Sleep(time.Second * 2)
	fmt.Println("~~ getting pods")
	secondPods := f.WaitForAllPodsReady(ctx, "app=sameimg")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, firstPods, secondPods)
}

package mage

import (
	"context"
	"fmt"

	"github.com/magefile/mage/mg"
)

type Test mg.Namespace

func (Test) Unit(ctx context.Context) error {
	fmt.Println("Running unit tests")

	client, err := getDaggerClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	src := getSource(client)

	unitTest := client.Container().From(GoImage)

	unitTest = unitTest.
		WithDirectory("/src", src).
		WithWorkdir("/src").
		With(goCache(client))

	unitTest = unitTest.WithExec([]string{
		"go", "test", "-v", "-count=1", "./...",
	})

	return printOutput(ctx, unitTest)
}

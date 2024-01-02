package mage

import (
	"context"
	"os"
	"os/exec"

	"github.com/magefile/mage/mg"
)

type Gomod mg.Namespace

func (gm Gomod) Tidy(ctx context.Context) {
	mg.CtxDeps(
		ctx,
		mg.F(gm.tidyDir, ""),
		mg.F(gm.tidyDir, "build/mage"),
		mg.F(gm.tidyDir, "integration"),
	)
}

func (Gomod) tidyDir(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = dir
	return cmd.Run()
}

package ostree

import (
	"context"
	"errors"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
)

type apiService struct{}

func newAPIService() *apiService {
	return &apiService{}
}

func (o *apiService) MirrorRepository(_ context.Context, _ string, _ int) (err error) {
	return werror.Wrap(gcode.ErrNotImplemented, errors.New("repository mirroring not yet supported"))
}

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

func (o *apiService) MirrorRepository(ctx context.Context, repository string, depth int) (err error) {
	//TODO implement me
	return werror.Wrap(gcode.ErrNotImplemented, errors.New("repository mirroring not yet supported"))
}

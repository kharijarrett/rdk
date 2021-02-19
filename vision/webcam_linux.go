package vision

import (
	"context"
	"fmt"
	"image"

	"github.com/blackjack/webcam"

	"github.com/edaniels/golog"
)

const (
	// from https://github.com/blackjack/webcam/blob/master/examples/http_mjpeg_streamer/webcam.go
	v4l2_pix_fmt_yuyv = 0x56595559
)

type WebcamSource struct {
	cam           *webcam.Webcam
	format        webcam.PixelFormat
	width, height uint32
}

func (s *WebcamSource) Close() error {
	return s.cam.Close()
}

func (s *WebcamSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	i, err := s.Next(ctx)
	return i, nil, err
}

func (s *WebcamSource) decode(frame []byte) image.Image {

	switch s.format {
	case v4l2_pix_fmt_yuyv:
		yuyv := image.NewYCbCr(image.Rect(0, 0, int(s.width), int(s.height)), image.YCbCrSubsampleRatio422)
		for i := range yuyv.Cb {
			ii := i * 4
			yuyv.Y[i*2] = frame[ii]
			yuyv.Y[i*2+1] = frame[ii+2]
			yuyv.Cb[i] = frame[ii+1]
			yuyv.Cr[i] = frame[ii+3]

		}
		return yuyv
	default:
		panic("invalid format ? - should be impossible")
	}
}

func (s *WebcamSource) Next(ctx context.Context) (image.Image, error) {

	err := s.cam.WaitForFrame(1)
	if err != nil {
		return nil, fmt.Errorf("couldn't get webcam frame: %s", err)
	}

	frame, err := s.cam.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("couldn't read webcam frame: %s", err)
	}

	if len(frame) == 0 {
		return nil, fmt.Errorf("why is frame empty")
	}

	return s.decode(frame), nil
}

func tryWebcamOpen(path string) (ImageDepthSource, error) {
	cam, err := webcam.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open webcam [%s] : %s", path, err)
	}

	formats := cam.GetSupportedFormats()
	_, ok := formats[v4l2_pix_fmt_yuyv]
	if !ok {
		return nil, fmt.Errorf("unsupported types %v", formats)
	}

	sizes := cam.GetSupportedFrameSizes(v4l2_pix_fmt_yuyv)
	bestSize := 0
	for idx, s := range sizes {
		if s.MaxWidth > sizes[bestSize].MaxWidth {
			bestSize = idx
		}
	}

	format, w, h, err := cam.SetImageFormat(v4l2_pix_fmt_yuyv, sizes[bestSize].MaxWidth, sizes[bestSize].MaxHeight)
	if err != nil {
		return nil, fmt.Errorf("cannot set image format: %s", err)
	}

	err = cam.StartStreaming()
	if err != nil {
		return nil, fmt.Errorf("cannot start webcam stream for %s : %s", path, err)
	}

	return &WebcamSource{cam, format, w, h}, nil
}

func NewWebcamSource(attrs map[string]string) (ImageDepthSource, error) {

	path := attrs["path"]

	if path != "" {
		return tryWebcamOpen(path)
	}

	for i := 0; i <= 20; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		s, err := tryWebcamOpen(path)
		if err == nil {
			golog.Global.Debugf("found webcam %s", path)
			return s, nil
		}
	}

	return nil, fmt.Errorf("could find no webcams")
}

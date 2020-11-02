package main

import (
	"fmt"
	"html/template"
	"image"
	"image/color"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"gocv.io/x/gocv"
)

var (
	err      error
	webcam   *gocv.VideoCapture
	frame_id int
)

var buffer = make(map[int][]byte)
var frame []byte
var mutex = &sync.Mutex{}
const MinimumArea = 3000

func main() {

	host := "0.0.0.0:3000"

	// open webcam
	if len(os.Args) < 2 {
		fmt.Println(">> device /dev/video0 (default)")
		webcam, err = gocv.VideoCaptureDevice(0)
	} else {
		fmt.Println(">> file/url :: " + os.Args[1])
		webcam, err = gocv.VideoCaptureFile(os.Args[1])
	}

	if err != nil {
		fmt.Printf("Error opening capture device: \n")
		return
	}
	defer webcam.Close()

	// start capturing
	go getframes()

	fmt.Println("Capturing. Open http://" + host)

	// start http server
	http.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		data := ""
		for {
			/*			fmt.Println("Frame ID: ", frame_id)
			 */mutex.Lock()
			data = "--frame\r\n  Content-Type: image/jpeg\r\n\r\n" + string(frame) + "\r\n\r\n"
			mutex.Unlock()
			time.Sleep(33 * time.Millisecond)
			w.Write([]byte(data))
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, _ := template.ParseFiles("index.html")
		t.Execute(w, "index")
	})

	log.Fatal(http.ListenAndServe(host, nil))
}

func getframes() {
	img := gocv.NewMat()
	defer img.Close()

	imgDelta := gocv.NewMat()
	defer imgDelta.Close()

	imgThresh := gocv.NewMat()
	defer imgThresh.Close()

	mog2 := gocv.NewBackgroundSubtractorMOG2()
	defer mog2.Close()

	status := "Ready"

	fmt.Printf("Start reading device: %v\n", 0)

	for {
		if ok := webcam.Read(&img); !ok {
			fmt.Printf("Device closed: %v\n", 0)
			return
		}
		if img.Empty() {
			continue
		}

		status = "Ready"
		statusColor := color.RGBA{0, 255, 0, 0}

		// first phase of cleaning up image, obtain foreground only
		mog2.Apply(img, &imgDelta)

		// remaining cleanup of the image to use for finding contours.
		// first use threshold
		gocv.Threshold(imgDelta, &imgThresh, 25, 255, gocv.ThresholdBinary)

		// then dilate
		kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
		defer kernel.Close()
		gocv.Dilate(imgThresh, &imgThresh, kernel)

		// now find contours
		contours := gocv.FindContours(imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)
		for i, c := range contours {
			area := gocv.ContourArea(c)
			if area < MinimumArea {
				continue
			}

			status = "Motion detected"
			statusColor = color.RGBA{255, 255, 0, 0}
			gocv.DrawContours(&img, contours, i, statusColor, 1)

			rect := gocv.BoundingRect(c)
			gocv.Rectangle(&img, rect, color.RGBA{70, 100, 255, 0}, 1)
		}

		gocv.PutText(&img, status, image.Pt(10, 20), gocv.FontHersheyPlain, 1.2, statusColor, 2)

		frame_id++
		gocv.Resize(img, &img, image.Point{}, float64(0.5), float64(0.5), 0)
		frame, _ = gocv.IMEncode(".jpg", img)
	}
}

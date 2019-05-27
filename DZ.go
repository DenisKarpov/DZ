package main

import (
	"bytes"
	"fmt"   
	"html/template" // пакет для логирования
	"image"
	"image/png"
	"io/ioutil"
	"math"
	"net/http" // пакет для поддержки HTTP протокола
	"strconv"
	"strings"
	"github.com/davvo/mercator"
	"github.com/fogleman/gg"
	geojson "github.com/paulmach/go.geojson"
)

const width, height = 256, 256
const mercatorMaxValue float64 = 20037508.342789244
const mercatorToCanvasScaleFactorX = float64(width) / (mercatorMaxValue)
const mercatorToCanvasScaleFactorY = float64(height) / (mercatorMaxValue)
var cache map[string][]byte

func indexTransfer(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("./index.html")
	if err != nil {
		fmt.Fprintf(w, err.Error())
	}

	t.ExecuteTemplate(w, "index", "hello")

}

func GetImage(w http.ResponseWriter, r *http.Request) {

	var err error

	key := r.URL.String()
	keys := strings.Split(key, "/")

	z, err := strconv.ParseFloat(keys[2], 64)
	x, err := strconv.ParseFloat(keys[3], 64)
	y, err := strconv.ParseFloat(keys[4], 64)

	var img image.Image
	var imgBytes []byte
	
	var featureCollectionJSON []byte
	var filePath = "rf.geojson"

	if cache[key] != nil {
		imgBytes = cache[key]
	} else {
		if featureCollectionJSON, err = ioutil.ReadFile(filePath); err != nil {
			fmt.Println(err.Error())
		}

		if img, err = CreatePNG(featureCollectionJSON, z, x, y); err != nil {
			fmt.Println(err.Error())
		}

		buffer := new(bytes.Buffer) 
		png.Encode(buffer, img)     
		imgBytes = buffer.Bytes()
		cache[key] = imgBytes
	}
	
	w.Write(imgBytes)
}
func CreatePNG(featureCollectionJSON []byte, z float64, x float64, y float64) (image.Image, error) {
	var coordinates [][][][][]float64
	var err error

	if coordinates, err = getUserCoordinates(featureCollectionJSON); err != nil {
		return nil, err
	}

	dc := gg.NewContext(width, height)
	scale := 1.0

	dc.InvertY()
	//отрисовка полигонов
	Polygon(dc, coordinates, func(polygonCoordinates [][]float64) {
		dc.SetRGB(0, 0, 1)
		PaintPolygonCoordinates(dc, polygonCoordinates, scale, dc.Fill, z, x, y)
	})
	//отрисовка полигонов
	dc.SetLineWidth(2)
	Polygon(dc, coordinates, func(polygonCoordinates [][]float64) {
		dc.SetRGB(0, 1, 0)
		PaintPolygonCoordinates(dc, polygonCoordinates, scale, dc.Stroke, z, x, y)
	})

	out := dc.Image()

	return out, nil
}
func getUserCoordinates(featureCollectionJSON []byte) ([][][][][]float64, error) {
	var featureCollection *geojson.FeatureCollection
	var err error

	if featureCollection, err = geojson.UnmarshalFeatureCollection(featureCollectionJSON); err != nil {
		return nil, err
	}
	var features = featureCollection.Features
	var coordinates [][][][][]float64
	for i := 0; i < len(features); i++ {
		coordinates = append(coordinates, features[i].Geometry.MultiPolygon)
	}
	return coordinates, nil
}
func Polygon(dc *gg.Context, coordinates [][][][][]float64, callback func([][]float64)) {
	for i := 0; i < len(coordinates); i++ {
		for j := 0; j < len(coordinates[i]); j++ {
			callback(coordinates[i][j][0])
		}
	}
}
func PaintPolygonCoordinates(dc *gg.Context, coordinates [][]float64, scale float64, method func(), z float64, xTile float64, yTile float64) {

	scale = scale * math.Pow(2, z)

	dx := float64(dc.Width())*(xTile) - 138.5*scale
	dy := float64(dc.Height())*(math.Pow(2, z)-1-yTile) - 128*scale

	for index := 0; index < len(coordinates)-1; index++ {
		x, y := mercator.LatLonToMeters(coordinates[index][1], convertX(coordinates[index][0]))

		x, y = centerPolygon(x, y)

		x *= mercatorToCanvasScaleFactorX * scale * 0.5
		y *= mercatorToCanvasScaleFactorY * scale * 0.5

		x -= dx
		y -= dy

		dc.LineTo(x, y)
	}
	dc.ClosePath()
	method()
}
func centerPolygon(x float64, y float64) (float64, float64) {
	var west = float64(1635093.15883866)

	if x > 0 {
		x -= west
	} else {
		x += 2*mercatorMaxValue - west
	}

	return x, y
}
func convertX(x float64) float64 {
	if x < 0 {
		x = x - 360
	}
	return x
}
func main() {
	cache = make(map[string][]byte, 0)

	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
	http.HandleFunc("/", indexTransfer)
	http.HandleFunc("/tile/", GetImage)

	http.ListenAndServe(":3000", nil)
}
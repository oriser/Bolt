package wolt

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/Jeffail/gabs/v2"
)

const (
	CoordinateKey  = "results.0.location.coordinates"
	PriceRangesKey = "results.0.delivery_specs.delivery_pricing"
)

type VenueDetails struct {
	ParsedOutput *gabs.Container
}

type Coordinate struct {
	Lat float64
	Lon float64
}

type PriceRanges struct {
	BasePrice      int             `json:"base_price"`
	DistanceRanges []DistanceRange `json:"distance_ranges"`
}

type DistanceRange struct {
	AddedPrice  int `json:"a"`
	MinDistance int `json:"min"`
	MaxDistance int `json:"max"`
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

// Distance function returns the distance (in meters) between two points of
//     a given longitude and latitude relatively accurately (using a spherical
//     approximation of the Earth) through the Haversin Distance Formula for
//     great arc distance on a sphere with accuracy for small distances
//
// point coordinates are supplied in degrees and converted into rad. in the func
//
// distance returned is METERS!!!!!!
// http://en.wikipedia.org/wiki/Haversine_formula
func Distance(first, second Coordinate) float64 {
	// convert to radians
	// must cast radius as float to multiply later
	var la1, lo1, la2, lo2, r float64
	la1 = first.Lat * math.Pi / 180
	lo1 = first.Lon * math.Pi / 180
	la2 = second.Lat * math.Pi / 180
	lo2 = second.Lon * math.Pi / 180

	r = 6378100 // Earth radius in METERS

	// calculate
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * r * math.Asin(math.Sqrt(h))
}

func CoordinateFromArray(arrayContainer *gabs.Container) (Coordinate, error) {
	c := Coordinate{}
	children := arrayContainer.Children()
	if len(children) != 2 {
		marshaled, _ := arrayContainer.MarshalJSON()
		return c, fmt.Errorf("%q doesn't have exactly 2 elements: %s", CoordinateKey, string(marshaled))
	}

	lat, ok := children[0].Data().(float64)
	if !ok {
		return c, fmt.Errorf("venue latitude is not float64: %v (%T)", children[0].Data(), children[0].Data())
	}
	c.Lat = lat

	lon, ok := children[1].Data().(float64)
	if !ok {
		return c, fmt.Errorf("venue lotitude is not float64: %v (%T)", children[1].Data(), children[1].Data())
	}
	c.Lon = lon

	return c, nil
}

func (v *VenueDetails) CalculateDeliveryRate(source Coordinate) (int, error) {
	location, err := v.Location()
	if err != nil {
		return 0, fmt.Errorf("get vennue location: %w", err)
	}

	pricesRange, err := v.PriceRanges()
	if err != nil {
		return 0, fmt.Errorf("get vennue prices range: %w", err)
	}

	distance := int(Distance(location, source))
	price := pricesRange.BasePrice
	for _, distanceRange := range pricesRange.DistanceRanges {
		if distance >= distanceRange.MinDistance && (distance < distanceRange.MaxDistance || distanceRange.MaxDistance == 0) {
			price += distanceRange.AddedPrice
			break
		}
	}

	return price / 100, nil
}

func (v *VenueDetails) PriceRanges() (PriceRanges, error) {
	p := PriceRanges{}
	if !v.ParsedOutput.ExistsP(PriceRangesKey) {
		return p, fmt.Errorf("no %q key in venue JSON", PriceRangesKey)
	}
	marshaled, err := v.ParsedOutput.Path(PriceRangesKey).MarshalJSON()
	if err != nil {
		return p, fmt.Errorf("remarshaling prices range: %w", err)
	}

	err = json.Unmarshal(marshaled, &p)
	if err != nil {
		return p, fmt.Errorf("unmarshal prices range: %w", err)
	}

	return p, nil
}

func (v *VenueDetails) Location() (Coordinate, error) {
	c := Coordinate{}
	if !v.ParsedOutput.ExistsP(CoordinateKey) {
		return c, fmt.Errorf("no %q key in venue JSON", CoordinateKey)
	}

	return CoordinateFromArray(v.ParsedOutput.Path(CoordinateKey))
}

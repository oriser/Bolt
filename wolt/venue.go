package wolt

import (
	"encoding/json"
	"fmt"
	"math"
)

const (
	CoordinateKey  = "results.0.location.coordinates"
	PriceRangesKey = "results.0.delivery_specs.delivery_pricing"
)

type VenueName struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type Venue struct {
	Location struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"location"`
	DeliverySpecs struct {
		DeliveryPricing PriceRanges `json:"delivery_pricing"`
	} `json:"delivery_specs"`
	Names []VenueName `json:"name"`
	Link  string      `json:"public_url"`
	City  string      `json:"city"`

	Name             string
	ParsedCoordinate Coordinate `json:"-"`
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

func ParseVenue(venuesJSON []byte) (*Venue, error) {
	var venues struct {
		Results []*Venue `json:"results"`
	}
	var err error
	if err = json.Unmarshal(venuesJSON, &venues); err != nil {
		return nil, fmt.Errorf("parse venue details JSON: %w", err)
	}

	if len(venues.Results) != 1 {
		return nil, fmt.Errorf("expected one venue to return but got #%d", len(venues.Results))
	}

	v := venues.Results[0]
	v.ParsedCoordinate, err = CoordinateFromArray(v.Location.Coordinates)
	if err != nil {
		return nil, fmt.Errorf("venue coordinate from array: %w", err)
	}

	for _, name := range v.Names {
		if name.Lang == "en" || v.Name == "" {
			v.Name = name.Value
		}
	}

	return v, nil
}

// haversin(θ) function
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

// Distance function returns the distance (in meters) between two points of
//
//	a given longitude and latitude relatively accurately (using a spherical
//	approximation of the Earth) through the Haversin Distance Formula for
//	great arc distance on a sphere with accuracy for small distances
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

func CoordinateFromArray(coordinateArr []float64) (Coordinate, error) {
	if len(coordinateArr) != 2 {
		return Coordinate{}, fmt.Errorf("expected exactly 2 items in coordinate array but got #%d", len(coordinateArr))
	}

	return Coordinate{
		Lat: coordinateArr[0],
		Lon: coordinateArr[1],
	}, nil
}

func (v *Venue) CalculateDeliveryRate(source Coordinate) (int, error) {
	distance := int(Distance(v.ParsedCoordinate, source))
	price := v.DeliverySpecs.DeliveryPricing.BasePrice
	for _, distanceRange := range v.DeliverySpecs.DeliveryPricing.DistanceRanges {
		if distance >= distanceRange.MinDistance && (distance < distanceRange.MaxDistance || distanceRange.MaxDistance == 0) {
			price += distanceRange.AddedPrice
			break
		}
	}

	return price / 100, nil
}

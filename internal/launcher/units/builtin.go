package units

// NewBuiltinRegistry returns a registry pre-loaded with all built-in unit groups.
// Groups are registered in a specific order so that symbol priority is correct:
// length before time ("m"→meters, not minutes; "min" added by time group with no conflict).
func NewBuiltinRegistry() *Registry {
	r := NewRegistry()
	r.AddGroup(lengthGroup())
	r.AddGroup(massGroup())
	r.AddGroup(temperatureGroup())
	r.AddGroup(speedGroup())
	r.AddGroup(dataGroup())
	r.AddGroup(timeGroup())
	r.AddGroup(areaGroup())
	r.AddGroup(volumeGroup())
	return r
}

func lengthGroup() *Group {
	return &Group{
		Name: "length",
		Units: []*Unit{
			{Name: "meter", Symbols: []string{"m", "meter", "meters"}, Factor: 1},
			{Name: "kilometer", Symbols: []string{"km", "kilometer", "kilometers"}, Factor: 1000},
			{Name: "centimeter", Symbols: []string{"cm", "centimeter", "centimeters"}, Factor: 0.01},
			{Name: "millimeter", Symbols: []string{"mm", "millimeter", "millimeters"}, Factor: 0.001},
			{Name: "micrometer", Symbols: []string{"μm", "um", "micrometer", "micron"}, Factor: 1e-6},
			{Name: "nanometer", Symbols: []string{"nm", "nanometer"}, Factor: 1e-9},
			{Name: "inch", Symbols: []string{"in", "inch", "inches"}, Factor: 0.0254, ExtraFormat: formatInchFraction},
			{Name: "foot", Symbols: []string{"ft", "foot", "feet"}, Factor: 0.3048},
			{Name: "yard", Symbols: []string{"yd", "yard", "yards"}, Factor: 0.9144},
			{Name: "mile", Symbols: []string{"mi", "mile", "miles"}, Factor: 1609.344},
			{Name: "nautical mile", Symbols: []string{"nmi", "nm2", "nautical"}, Factor: 1852},
			{Name: "light-year", Symbols: []string{"ly", "lightyear", "light-year"}, Factor: 9.4607304725808e15},
		},
	}
}

func massGroup() *Group {
	return &Group{
		Name: "mass",
		Units: []*Unit{
			{Name: "kilogram", Symbols: []string{"kg", "kilogram", "kilograms"}, Factor: 1},
			{Name: "gram", Symbols: []string{"g", "gram", "grams"}, Factor: 0.001},
			{Name: "milligram", Symbols: []string{"mg", "milligram"}, Factor: 1e-6},
			{Name: "microgram", Symbols: []string{"μg", "ug", "microgram"}, Factor: 1e-9},
			{Name: "tonne", Symbols: []string{"t", "tonne", "metric ton"}, Factor: 1000},
			{Name: "pound", Symbols: []string{"lb", "lbs", "pound", "pounds"}, Factor: 0.45359237},
			{Name: "ounce", Symbols: []string{"oz", "ounce", "ounces"}, Factor: 0.028349523125},
			{Name: "stone", Symbols: []string{"st", "stone"}, Factor: 6.35029318},
		},
	}
}

// temperatureGroup uses Kelvin as base.
// toBase = value*Factor + Offset; fromBase = (base - Offset) / Factor
func temperatureGroup() *Group {
	return &Group{
		Name: "temperature",
		Units: []*Unit{
			{Name: "kelvin", Symbols: []string{"k", "kelvin", "°k"}, Factor: 1, Offset: 0},
			{Name: "celsius", Symbols: []string{"c", "°c", "celsius"}, Factor: 1, Offset: 273.15},
			// toBase(F) = F * 5/9 + 255.3722... = F * (5/9) + (273.15 - 32*5/9)
			{Name: "fahrenheit", Symbols: []string{"f", "°f", "fahrenheit"}, Factor: 5.0 / 9.0, Offset: 273.15 - 32.0*5.0/9.0},
		},
	}
}

func speedGroup() *Group {
	// base: meters per second
	return &Group{
		Name: "speed",
		Units: []*Unit{
			{Name: "m/s", Symbols: []string{"mps", "m/s"}, Factor: 1},
			{Name: "km/h", Symbols: []string{"kph", "kmh", "km/h"}, Factor: 1.0 / 3.6},
			{Name: "mph", Symbols: []string{"mph"}, Factor: 0.44704},
			{Name: "knot", Symbols: []string{"knot", "kn", "kt"}, Factor: 0.514444},
			{Name: "ft/s", Symbols: []string{"fps", "ft/s"}, Factor: 0.3048},
		},
	}
}

func dataGroup() *Group {
	// base: bytes
	return &Group{
		Name: "data",
		Units: []*Unit{
			{Name: "byte", Symbols: []string{"b", "byte", "bytes"}, Factor: 1},
			{Name: "bit", Symbols: []string{"bit", "bits"}, Factor: 0.125},
			{Name: "kilobyte", Symbols: []string{"kb", "kilobyte"}, Factor: 1e3},
			{Name: "megabyte", Symbols: []string{"mb", "megabyte"}, Factor: 1e6},
			{Name: "gigabyte", Symbols: []string{"gb", "gigabyte"}, Factor: 1e9},
			{Name: "terabyte", Symbols: []string{"tb", "terabyte"}, Factor: 1e12},
			{Name: "petabyte", Symbols: []string{"pb", "petabyte"}, Factor: 1e15},
			{Name: "kibibyte", Symbols: []string{"kib", "kibibyte"}, Factor: 1024},
			{Name: "mebibyte", Symbols: []string{"mib", "mebibyte"}, Factor: 1024 * 1024},
			{Name: "gibibyte", Symbols: []string{"gib", "gibibyte"}, Factor: 1024 * 1024 * 1024},
			{Name: "tebibyte", Symbols: []string{"tib", "tebibyte"}, Factor: 1024 * 1024 * 1024 * 1024},
		},
	}
}

func timeGroup() *Group {
	// base: seconds
	return &Group{
		Name: "time",
		Units: []*Unit{
			{Name: "second", Symbols: []string{"s", "sec", "second", "seconds"}, Factor: 1},
			{Name: "millisecond", Symbols: []string{"ms", "millisecond"}, Factor: 0.001},
			{Name: "microsecond", Symbols: []string{"μs", "us", "microsecond"}, Factor: 1e-6},
			{Name: "nanosecond", Symbols: []string{"ns", "nanosecond"}, Factor: 1e-9},
			{Name: "minute", Symbols: []string{"min", "minute", "minutes"}, Factor: 60},
			{Name: "hour", Symbols: []string{"h", "hr", "hour", "hours"}, Factor: 3600},
			{Name: "day", Symbols: []string{"d", "day", "days"}, Factor: 86400},
			{Name: "week", Symbols: []string{"wk", "week", "weeks"}, Factor: 604800},
			{Name: "month", Symbols: []string{"mo", "month", "months"}, Factor: 2629800}, // avg
			{Name: "year", Symbols: []string{"yr", "year", "years"}, Factor: 31557600},   // Julian
		},
	}
}

func areaGroup() *Group {
	// base: square meters
	return &Group{
		Name: "area",
		Units: []*Unit{
			{Name: "m²", Symbols: []string{"m2", "sqm"}, Factor: 1},
			{Name: "km²", Symbols: []string{"km2", "sqkm"}, Factor: 1e6},
			{Name: "cm²", Symbols: []string{"cm2"}, Factor: 1e-4},
			{Name: "mm²", Symbols: []string{"mm2"}, Factor: 1e-6},
			{Name: "in²", Symbols: []string{"in2", "sqin"}, Factor: 6.4516e-4},
			{Name: "ft²", Symbols: []string{"ft2", "sqft"}, Factor: 0.09290304},
			{Name: "yd²", Symbols: []string{"yd2", "sqyd"}, Factor: 0.83612736},
			{Name: "mi²", Symbols: []string{"mi2", "sqmi"}, Factor: 2589988.110336},
			{Name: "acre", Symbols: []string{"acre", "acres"}, Factor: 4046.8564224},
			{Name: "hectare", Symbols: []string{"ha", "hectare"}, Factor: 10000},
		},
	}
}

func volumeGroup() *Group {
	// base: cubic meters (m³)
	return &Group{
		Name: "volume",
		Units: []*Unit{
			{Name: "m³", Symbols: []string{"m3", "cbm"}, Factor: 1},
			{Name: "liter", Symbols: []string{"l", "liter", "litre", "liters"}, Factor: 0.001},
			{Name: "milliliter", Symbols: []string{"ml", "milliliter", "millilitre"}, Factor: 1e-6},
			{Name: "cm³", Symbols: []string{"cm3", "cc"}, Factor: 1e-6},
			{Name: "ft³", Symbols: []string{"ft3", "cbft"}, Factor: 0.028316846592},
			{Name: "in³", Symbols: []string{"in3"}, Factor: 1.6387064e-5},
			{Name: "US gallon", Symbols: []string{"gal", "gallon"}, Factor: 0.003785411784},
			{Name: "US fl oz", Symbols: []string{"floz", "fl oz"}, Factor: 2.95735296875e-5},
			{Name: "cup", Symbols: []string{"cup", "cups"}, Factor: 2.365882365e-4},
			{Name: "tablespoon", Symbols: []string{"tbsp", "tablespoon"}, Factor: 1.47867647825e-5},
			{Name: "teaspoon", Symbols: []string{"tsp", "teaspoon"}, Factor: 4.92892159375e-6},
		},
	}
}

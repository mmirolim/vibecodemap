package roads

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

type District struct {
	ID   string
	Code string
	Name string
}

type Relation struct {
	ID          string
	FromSubject string
	ToSubject   string
	Family      string
	Execution   string
	Count       int
	Strength    float64
}

type Endpoint struct {
	DistrictID   string
	DistrictCode string
	OppositeCode string
}

type Lane struct {
	FromDistrict string
	ToDistrict   string
	Family       string
	Execution    string
	Count        int
	Strength     float64
	RelationIDs  []string
}

type Feeder struct {
	Subject        string
	District       string
	RoadID         string
	RemoteDistrict string
	Family         string
	Direction      string
	Count          int
	Strength       float64
}

type Road struct {
	ID         string
	A          Endpoint
	B          Endpoint
	Lanes      []Lane
	TotalCount int
	Strength   float64
}

type Network struct {
	Roads          []Road
	Feeders        []Feeder
	LocalRelations []string
	Unrouted       []string
}

type roadBuilder struct {
	road  Road
	lanes map[string]*Lane
}

// Build aggregates inter-district relations into one physical road per
// district pair while preserving directional, family, and execution lanes. It
// also emits thin local feeders so a renderer can connect buildings to road
// ports without redrawing every long relation independently.
func Build(districts []District, subjectDistrict map[string]string, relations []Relation) (Network, error) {
	byID := make(map[string]District, len(districts))
	codes := make(map[string]string, len(districts))
	for _, district := range districts {
		if district.ID == "" || district.Code == "" {
			return Network{}, fmt.Errorf("district id and code are required: %+v", district)
		}
		if _, exists := byID[district.ID]; exists {
			return Network{}, fmt.Errorf("duplicate district id %q", district.ID)
		}
		if previous, exists := codes[district.Code]; exists {
			return Network{}, fmt.Errorf("district code %q is shared by %q and %q", district.Code, previous, district.ID)
		}
		byID[district.ID] = district
		codes[district.Code] = district.ID
	}

	builders := make(map[string]*roadBuilder)
	roadKeysByID := make(map[string]string)
	feeders := make(map[string]*Feeder)
	network := Network{}
	relationIDs := make(map[string]struct{}, len(relations))

	for index, relation := range relations {
		if relation.ID == "" {
			return Network{}, fmt.Errorf("relation %d has no id", index)
		}
		if _, duplicate := relationIDs[relation.ID]; duplicate {
			return Network{}, fmt.Errorf("duplicate relation id %q", relation.ID)
		}
		relationIDs[relation.ID] = struct{}{}
		if relation.FromSubject == "" || relation.ToSubject == "" {
			return Network{}, fmt.Errorf("relation %q has an empty subject endpoint", relation.ID)
		}
		if relation.Family == "" || relation.Execution == "" {
			return Network{}, fmt.Errorf("relation %q must declare family and execution", relation.ID)
		}
		if relation.Count < 0 {
			return Network{}, fmt.Errorf("relation %q has a negative count", relation.ID)
		}
		if math.IsNaN(relation.Strength) || math.IsInf(relation.Strength, 0) || relation.Strength < 0 {
			return Network{}, fmt.Errorf("relation %q has invalid strength", relation.ID)
		}
		fromDistrict, fromOK := subjectDistrict[relation.FromSubject]
		toDistrict, toOK := subjectDistrict[relation.ToSubject]
		if !fromOK || !toOK {
			network.Unrouted = append(network.Unrouted, relation.ID)
			continue
		}
		from, fromExists := byID[fromDistrict]
		to, toExists := byID[toDistrict]
		if !fromExists || !toExists {
			network.Unrouted = append(network.Unrouted, relation.ID)
			continue
		}
		if from.ID == to.ID {
			network.LocalRelations = append(network.LocalRelations, relation.ID)
			continue
		}

		left, right := from, to
		if right.ID < left.ID {
			left, right = right, left
		}
		roadKey := left.ID + "\x00" + right.ID
		builder := builders[roadKey]
		if builder == nil {
			identifier := roadID(left.Code, right.Code)
			if previous, collision := roadKeysByID[identifier]; collision && previous != roadKey {
				return Network{}, fmt.Errorf("district codes produce colliding road id %q", identifier)
			}
			roadKeysByID[identifier] = roadKey
			builder = &roadBuilder{
				road: Road{
					ID: identifier,
					A:  Endpoint{DistrictID: left.ID, DistrictCode: left.Code, OppositeCode: right.Code},
					B:  Endpoint{DistrictID: right.ID, DistrictCode: right.Code, OppositeCode: left.Code},
				},
				lanes: make(map[string]*Lane),
			}
			builders[roadKey] = builder
		}

		count := relation.Count
		if count <= 0 {
			count = 1
		}
		strength := relation.Strength
		if strength <= 0 {
			strength = float64(count)
		}
		laneKey := strings.Join([]string{from.ID, to.ID, relation.Family, relation.Execution}, "\x00")
		lane := builder.lanes[laneKey]
		if lane == nil {
			lane = &Lane{
				FromDistrict: from.ID,
				ToDistrict:   to.ID,
				Family:       relation.Family,
				Execution:    relation.Execution,
			}
			builder.lanes[laneKey] = lane
		}
		lane.Count += count
		lane.Strength += strength
		lane.RelationIDs = append(lane.RelationIDs, relation.ID)
		builder.road.TotalCount += count
		builder.road.Strength += strength

		addFeeder(feeders, relation.FromSubject, from.ID, builder.road.ID, to.ID, relation.Family, "out", count, strength)
		addFeeder(feeders, relation.ToSubject, to.ID, builder.road.ID, from.ID, relation.Family, "in", count, strength)
	}

	roadKeys := make([]string, 0, len(builders))
	for key := range builders {
		roadKeys = append(roadKeys, key)
	}
	sort.Strings(roadKeys)
	for _, key := range roadKeys {
		builder := builders[key]
		laneKeys := make([]string, 0, len(builder.lanes))
		for laneKey := range builder.lanes {
			laneKeys = append(laneKeys, laneKey)
		}
		sort.Strings(laneKeys)
		for _, laneKey := range laneKeys {
			lane := *builder.lanes[laneKey]
			sort.Strings(lane.RelationIDs)
			builder.road.Lanes = append(builder.road.Lanes, lane)
		}
		network.Roads = append(network.Roads, builder.road)
	}

	feederKeys := make([]string, 0, len(feeders))
	for key := range feeders {
		feederKeys = append(feederKeys, key)
	}
	sort.Strings(feederKeys)
	for _, key := range feederKeys {
		network.Feeders = append(network.Feeders, *feeders[key])
	}
	sort.Strings(network.LocalRelations)
	sort.Strings(network.Unrouted)
	return network, nil
}

func addFeeder(
	feeders map[string]*Feeder,
	subject, district, road, remote, family, direction string,
	count int,
	strength float64,
) {
	key := strings.Join([]string{subject, road, remote, family, direction}, "\x00")
	feeder := feeders[key]
	if feeder == nil {
		feeder = &Feeder{
			Subject:        subject,
			District:       district,
			RoadID:         road,
			RemoteDistrict: remote,
			Family:         family,
			Direction:      direction,
		}
		feeders[key] = feeder
	}
	feeder.Count += count
	feeder.Strength += strength
}

func roadID(leftCode, rightCode string) string {
	return "road." + slug(leftCode) + "-" + slug(rightCode)
}

func slug(value string) string {
	var result strings.Builder
	lastDash := false
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}

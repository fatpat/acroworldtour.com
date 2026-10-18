package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"awt/internal/civl"
	"awt/internal/domain"
	"awt/internal/store/gen"
)

// Pilot is a pilot with its AWT years (spec 02 §2.1). SocialLinks and
// Sponsors are the stored JSON arrays.
type Pilot struct {
	CivlID          int
	Name            string
	Country         string
	About           string
	CivlLink        string
	PhotoURL        string
	PhotoHighresURL string
	BackgroundURL   string
	Rank            int
	Gender          domain.Gender
	AWTYears        []int
	SocialLinks     json.RawMessage
	Sponsors        json.RawMessage
	UpdatedAt       string
}

func pilotFromRow(p gen.Pilot, years []int64) Pilot {
	out := Pilot{
		CivlID: int(p.Civlid), Name: p.Name, Country: p.Country, About: p.About, CivlLink: p.CivlLink,
		PhotoURL: p.PhotoUrl, PhotoHighresURL: p.PhotoHighresUrl, BackgroundURL: p.BackgroundUrl,
		Rank: int(p.Rank), Gender: domain.Gender(p.Gender), AWTYears: []int{},
		SocialLinks: json.RawMessage(p.SocialLinks), Sponsors: json.RawMessage(p.Sponsors), UpdatedAt: p.UpdatedAt.String,
	}
	for _, y := range years {
		out.AWTYears = append(out.AWTYears, int(y))
	}
	return out
}

func loadPilot(ctx context.Context, q *gen.Queries, civlid int) (Pilot, error) {
	p, err := q.GetPilot(ctx, int64(civlid))
	if err != nil {
		return Pilot{}, notFound(err, fmt.Sprintf("pilot %d", civlid))
	}
	years, err := q.ListPilotAwtYears(ctx, p.Civlid)
	if err != nil {
		return Pilot{}, fmt.Errorf("list awt years of pilot %d: %w", civlid, err)
	}
	return pilotFromRow(p, years), nil
}

// GetPilot returns a pilot by CIVL id.
func (s *Service) GetPilot(ctx context.Context, civlid int) (Pilot, error) {
	return loadPilot(ctx, s.read(), civlid)
}

// ListPilots returns every pilot, best CIVL rank first.
func (s *Service) ListPilots(ctx context.Context) ([]Pilot, error) {
	q := s.read()
	rows, err := q.ListPilots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pilots: %w", err)
	}
	out := make([]Pilot, 0, len(rows))
	for _, p := range rows {
		years, err := q.ListPilotAwtYears(ctx, p.Civlid)
		if err != nil {
			return nil, fmt.Errorf("list awt years of pilot %d: %w", p.Civlid, err)
		}
		out = append(out, pilotFromRow(p, years))
	}
	return out, nil
}

type socialLink struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

type sponsor struct {
	Name string `json:"name"`
	Link string `json:"link,omitempty"`
	Img  string `json:"img"`
}

// ImportPilotFromCIVL creates or refreshes a pilot from its CIVL page. Only
// identity fields are written: rank, gender and AWT years are kept
// (spec 02 §2.1); a new pilot is unranked (9999) until RefreshRankings.
func (s *Service) ImportPilotFromCIVL(ctx context.Context, civlid int) (Pilot, error) {
	if civlid < 1 {
		return Pilot{}, invalid(domain.FieldError{Field: "civlid", Msg: "must be a positive CIVL id"})
	}
	d, err := s.civl.FetchPilot(ctx, civlid) // network call outside the transaction
	if errors.Is(err, civl.ErrNotFound) {
		return Pilot{}, fmt.Errorf("CIVL pilot %d: %w", civlid, ErrNotFound)
	}
	if err != nil {
		return Pilot{}, fmt.Errorf("fetch CIVL pilot %d: %w", civlid, err)
	}
	links := make([]socialLink, 0, len(d.SocialLinks))
	for _, l := range d.SocialLinks {
		links = append(links, socialLink(l))
	}
	sponsors := make([]sponsor, 0, len(d.Sponsors))
	for _, sp := range d.Sponsors {
		sponsors = append(sponsors, sponsor(sp))
	}
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return Pilot{}, err
	}
	sponsorsJSON, err := json.Marshal(sponsors)
	if err != nil {
		return Pilot{}, err
	}
	var out Pilot
	err = s.db.Tx(ctx, func(q *gen.Queries) error {
		err := q.UpsertPilotIdentity(ctx, gen.UpsertPilotIdentityParams{
			Civlid: int64(civlid), Name: d.Name, Country: d.Country, About: d.About, CivlLink: d.CivlLink,
			PhotoUrl: d.Photo, PhotoHighresUrl: d.PhotoHighres, BackgroundUrl: d.BackgroundPicture,
			SocialLinks: string(linksJSON), Sponsors: string(sponsorsJSON), UpdatedAt: nullString(now()),
		})
		if err != nil {
			return fmt.Errorf("save pilot %d: %w", civlid, err)
		}
		out, err = loadPilot(ctx, q, civlid)
		return err
	})
	return out, err
}

// RefreshRankings updates every pilot's CIVL world rank; pilots absent from
// the ranking become unranked (9999).
func (s *Service) RefreshRankings(ctx context.Context) error {
	ranks, err := s.civl.FetchRankings(ctx)
	if err != nil {
		return fmt.Errorf("fetch CIVL rankings: %w", err)
	}
	if len(ranks) == 0 {
		// an empty export would unrank every pilot
		return errors.New("CIVL returned no rankings")
	}
	return s.db.Tx(ctx, func(q *gen.Queries) error {
		pilots, err := q.ListPilots(ctx)
		if err != nil {
			return fmt.Errorf("list pilots: %w", err)
		}
		for _, p := range pilots {
			rank, ok := ranks[int(p.Civlid)]
			if !ok {
				rank = civl.UnrankedRank
			}
			if int64(rank) == p.Rank {
				continue
			}
			if err := q.UpdatePilotRank(ctx, gen.UpdatePilotRankParams{Rank: int64(rank), Civlid: p.Civlid}); err != nil {
				return fmt.Errorf("update rank of pilot %d: %w", p.Civlid, err)
			}
		}
		return nil
	})
}

// ToggleGender switches man and woman; none stays none (spec 02 §2.1).
func (s *Service) ToggleGender(ctx context.Context, civlid int) (domain.Gender, error) {
	var g domain.Gender
	var topics []string
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		p, err := q.GetPilot(ctx, int64(civlid))
		if err != nil {
			return notFound(err, fmt.Sprintf("pilot %d", civlid))
		}
		switch g = domain.Gender(p.Gender); g {
		case domain.GenderMan:
			g = domain.GenderWoman
		case domain.GenderWoman:
			g = domain.GenderMan
		default:
			return nil
		}
		if err := q.UpdatePilotGender(ctx, gen.UpdatePilotGenderParams{Gender: string(g), Civlid: p.Civlid}); err != nil {
			return fmt.Errorf("update gender of pilot %d: %w", civlid, err)
		}
		// the women lists change (spec 03 §7.1)
		ids, err := pilotCompetitions(ctx, q, civlid, 0)
		if err != nil {
			return err
		}
		topics, err = refreshCompetitions(ctx, q, ids)
		return err
	})
	if err != nil {
		return g, err
	}
	s.publish(KindCompetition, topics...)
	return g, nil
}

// SetAWTYear makes the pilot an AWT pilot for year (awt true) or an AWQ
// pilot (awt false). Years are 1990-2100.
func (s *Service) SetAWTYear(ctx context.Context, civlid, year int, awt bool) error {
	if year < 1990 || year > 2100 {
		return invalid(domain.FieldError{Field: "year", Msg: "must be between 1990 and 2100"})
	}
	var topics []string
	err := s.db.Tx(ctx, func(q *gen.Queries) error {
		if _, err := q.GetPilot(ctx, int64(civlid)); err != nil {
			return notFound(err, fmt.Sprintf("pilot %d", civlid))
		}
		var err error
		if awt {
			err = q.AddPilotAwtYear(ctx, gen.AddPilotAwtYearParams{Civlid: int64(civlid), Year: int64(year)})
		} else {
			err = q.RemovePilotAwtYear(ctx, gen.RemovePilotAwtYearParams{Civlid: int64(civlid), Year: int64(year)})
		}
		if err != nil {
			return fmt.Errorf("set AWT year %d of pilot %d: %w", year, civlid, err)
		}
		// the mark type and the awt/awq lists of that year's competitions
		// follow the AWT status (decision 0006)
		ids, err := pilotCompetitions(ctx, q, civlid, year)
		if err != nil {
			return err
		}
		topics, err = refreshCompetitions(ctx, q, ids)
		return err
	})
	if err != nil {
		return err
	}
	s.publish(KindCompetition, topics...)
	return nil
}

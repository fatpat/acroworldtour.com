// Package templates holds the templ components of the printable exports. They
// take pre-formatted view models so that formatting stays in package export.
package templates

// Person is one pilot cell. Href is empty when the name is not linked.
type Person struct {
	Name    string
	Country string
	Href    string
}

// Participant is a solo pilot (Members empty) or a synchro team.
type Participant struct {
	Person
	Members []Person
}

// Page is the common header data.
type Page struct {
	CompName string
	Heading  string
	Updated  string // preformatted, empty to omit
	Solo     bool
}

type RunScore struct{ Label, Score string }

type ResultsRow struct {
	Rank  int
	P     Participant
	Runs  []RunScore
	Total string
}

type ResultsTable struct {
	Title string
	Rows  []ResultsRow
}

type ResultsView struct {
	Page
	Tables []ResultsTable
}

type RunRow struct {
	Rank       int
	P          Participant
	Warnings   []string
	Malus      string // empty when none
	Technical  string
	Choreo     string
	Landing    string
	Synchro    string
	Technicity string
	Bonus      string
	Score      string
}

type RunTable struct {
	Title string
	Rows  []RunRow
}

type RunResultsView struct {
	Page
	Tables []RunTable
}

type StartingRow struct {
	Position int
	P        Participant
}

type StartingTable struct {
	Title string
	Rows  []StartingRow
}

type StartingOrderView struct {
	Page
	Tables []StartingTable
}

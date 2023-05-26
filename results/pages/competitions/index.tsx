import classNames from "classnames";
import { useEffect, useState } from "react";
import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import { useLayout } from "@/components/layout/layoutContext";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];
type Season = components["schemas"]["SeasonExport"];

interface YearSelectorProps {
  years: number[];
  // eslint-disable-next-line no-unused-vars
  onChange: (year: number) => void;
}

const Competitions = () => {
  const [selectedYear, setSelectedYear] = useState<number | null>(null);

  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Competitions");
    setPageDescription(`All the competitions of the Acro World Tour.`);
    setHeaderTitle("Competitions");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("competitions");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const { data: seasons, error: seasonsError } = useSWR<Season[]>(
    `${API_URL}/seasons`,
    fetcher
  );

  const { data: competitions, error: competitionsError } = useSWR<
    Competition[]
  >(`${API_URL}/competitions`, fetcher);

  if (competitionsError || seasonsError) return <FetchError />;
  if (!competitions || !seasons) return <FetchLoading />;

  const filteredCompetitions = selectedYear
    ? competitions.filter((competition) => {
        const startYear = new Date(competition.start_date).getFullYear();
        const endYear = new Date(competition.end_date).getFullYear();
        return startYear === selectedYear || endYear === selectedYear;
      })
    : competitions;

  const soloSeasons = seasons.filter(
    (season) =>
      season.type === "solo" &&
      season.competitions.some((comp) =>
        filteredCompetitions.some(
          (filteredComp) => filteredComp.code === comp.code
        )
      )
  );

  const offSeasonCompetitions = filteredCompetitions.filter(
    (competition) => competition.seasons.length === 0
  );

  const YearSelector: React.FC<YearSelectorProps> = ({ years, onChange }) => {
    const handleYearChange = ({
      target,
    }: React.ChangeEvent<HTMLSelectElement>) => {
      const year = parseInt(target.value);
      setSelectedYear(year);
      onChange(year);
    };

    return (
      <header className="font-semibold opacity-75">
        <h2
          className={classNames(
            "flex items-baseline",
            !selectedYear && "flex-row-reverse"
          )}
        >
          <label htmlFor="year-selector">
            {selectedYear
              ? filteredCompetitions.length === 0
                ? `No competitions in `
                : `Competitions in `
              : "competitions"}
          </label>
          <select
            id="year-selector"
            title="Select a year"
            value={selectedYear ?? ""}
            className="border-0 font-sans text-xl sm:text-2xl"
            onChange={handleYearChange}
          >
            <option value="" className="text-lg text-awtgrey-600">
              All
            </option>
            {years.map((year) => (
              <option
                key={year}
                value={year}
                className="text-lg text-awtgrey-600"
              >
                {year}
              </option>
            ))}
          </select>
        </h2>
      </header>
    );
  };

  return (
    <>
      <YearSelector
        years={[
          1970, // TESTING ONLY
          ...new Set(
            competitions.map((comp) => new Date(comp.start_date).getFullYear())
          ),
        ]}
        onChange={setSelectedYear}
      />

      {soloSeasons
        .sort((a, b) => b.year - a.year || b.code.localeCompare(a.code))
        .map((season) => {
          const { code, competitions, name } = season;
          return (
            <>
              <h3 className="mb-6 mt-6 opacity-50">{name}</h3>
              <section key={code} className={classNames("wrapper mb-8")}>
                {competitions.map((competition) => (
                  <CompetitionCard
                    key={competition.code}
                    competition={competition}
                  />
                ))}
              </section>
            </>
          );
        })}
      {offSeasonCompetitions.length > 0 && (
        <>
          <h3 className="mb-6 mt-6 opacity-50">Off Season</h3>
          <section className={classNames("wrapper mb-8")}>
            {offSeasonCompetitions
              .sort((a, b) => b.start_date.localeCompare(a.start_date))
              .map((competition) => (
                <CompetitionCard
                  key={competition.code}
                  competition={competition}
                />
              ))}
          </section>
        </>
      )}
    </>
  );
};

export default Competitions;

import classNames from "classnames";
import { ChangeEvent, Fragment, useEffect, useState } from "react";
import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import { useLayout } from "@/components/layout/layoutContext";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";

type Competition = components["schemas"]["CompetitionPublicExport"];
type Season = components["schemas"]["SeasonExport"];

const currentYear = new Date().getFullYear();

const Competitions = () => {
  const [selectedYear, setSelectedYear] = useState(currentYear);

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

  const {
    data: seasons,
    error: seasonsError,
    isLoading: seasonsLoading,
  } = useSWR<Season[]>(`${API_URL}/seasons`);

  const {
    data: competitions,
    error: competitionsError,
    isLoading: competitionsLoading,
  } = useSWR<Competition[]>(`${API_URL}/competitions`);

  if (seasonsLoading || competitionsLoading) return <FetchLoading />;
  if (competitionsError || seasonsError) return <FetchError />;
  if (!competitions || !seasons) return <h2>Competitions not found</h2>;

  const filteredCompetitions = competitions.filter((competition) => {
    const startYear = new Date(competition.start_date).getFullYear();
    const endYear = new Date(competition.end_date).getFullYear();
    return startYear === selectedYear || endYear === selectedYear;
  });

  const soloSeasons = seasons.filter(
    (season) =>
      season.type === "solo" &&
      season.competitions.some((comp) =>
        filteredCompetitions.some(
          (filteredComp) => filteredComp.code === comp.code
        )
      )
  );

  soloSeasons.sort(
    (a, b) =>
      b.year - a.year ||
      (a.index || 999) - (b.index || 999) ||
      a.name.localeCompare(b.name)
  );

  const offSeasonCompetitions = filteredCompetitions.filter(
    (competition) => competition.seasons.length === 0
  );

  offSeasonCompetitions.sort((a, b) =>
    b.start_date.localeCompare(a.start_date)
  );

  const years = [
    ...new Set(
      competitions.flatMap((comp) => [
        new Date(comp.start_date).getFullYear(),
        new Date(comp.end_date).getFullYear(),
      ])
    ).add(currentYear),
  ].sort((a, b) => b - a);

  const YearSelector = ({ years }: { years: number[] }) => {
    const handleYearChange = ({ target }: ChangeEvent<HTMLSelectElement>) => {
      const year = parseInt(target.value);
      setSelectedYear(year);
    };

    return (
      <header className="font-semibold opacity-95">
        <h2 className="-mt-2 flex items-baseline">
          <label htmlFor="year-selector">
            {filteredCompetitions.length === 0
              ? `No competitions in `
              : `Competitions in `}
          </label>
          <select
            id="year-selector"
            title="Select a year"
            value={selectedYear ?? ""}
            className="border-0 font-sans text-lg sm:text-xl"
            onChange={handleYearChange}
          >
            {years.map((year) => (
              <option
                key={year}
                value={year}
                className="text-base text-awt-dark-600"
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
      <YearSelector years={years} />

      {soloSeasons.map((season) => {
        const { code, competitions, name } = season;
        competitions.sort(
          (a, b) =>
            new Date(a.start_date).getTime() - new Date(b.start_date).getTime()
        );

        return (
          <Fragment key={code}>
            <h3 className="mb-6 mt-4 opacity-80">{name}</h3>
            <section key={code} className={classNames("wrapper mb-8")}>
              {competitions.map((competition) => (
                <CompetitionCard
                  key={competition.code}
                  competition={competition}
                />
              ))}
            </section>
          </Fragment>
        );
      })}
      {offSeasonCompetitions.length > 0 && (
        <>
          <h3 className="mb-6 mt-6 opacity-80">Off Season</h3>
          <section className={classNames("wrapper mb-8")}>
            {offSeasonCompetitions.map((competition) => (
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

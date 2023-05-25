import classNames from "classnames";
import { useEffect } from "react";
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

const Competitions = () => {
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

  const soloSeasons = seasons.filter(
    (season) => season.type === "solo" && season.competitions.length > 0
  );

  const offSeasonCompetitions = competitions.filter(
    (competition) => competition.seasons.length === 0
  );

  return (
    <>
      <h2 className="-mb-6">All Competitions</h2>
      {soloSeasons
        .sort((a, b) => b.year - a.year || b.code.localeCompare(a.code))
        .map((season) => {
          const {
            code,
            competitions,
            name,
            year,
            image,
            results,
            number_of_pilots,
          } = season;
          return (
            <>
              <h3 className="mb-6 mt-12 opacity-50">{name}</h3>
              <section key={code} className={classNames("wrapper")}>
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
      <h3 className="mb-6 mt-12 opacity-50">Off Season</h3>
      <section className={classNames("wrapper")}>
        {offSeasonCompetitions
          .sort((a, b) => b.start_date.localeCompare(a.start_date))
          .map((competition) => (
            <CompetitionCard key={competition.code} competition={competition} />
          ))}
      </section>
    </>
  );
};

export default Competitions;

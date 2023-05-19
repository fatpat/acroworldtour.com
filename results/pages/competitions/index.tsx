import classNames from "classnames";
import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];
type Season = components["schemas"]["SeasonExport"];

const Competitions = () => {
  const { data: seasons, error: seasonsError } = useSWR<Season[]>(
    `${API_URL}/seasons`,
    fetcher
  );

  const { data: competitions, error: competitionsError } = useSWR<
    Competition[]
  >(`${API_URL}/competitions`, fetcher);

  if (competitionsError || seasonsError) return <FetchError />;
  if (!competitions || !seasons) return <FetchLoading />;

  const soloSeasons = seasons.filter((season) => season.type === "solo");
  const orphanCompetitions = competitions.filter(
    (competition) => competition.seasons.length === 0
  );

  return (
    <>
      <h2 className="pt-4">Competitions</h2>
      <section className={classNames("flex flex-wrap")}>
        {soloSeasons.map((season) => {
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
            <section key={code}>
              <h3>{name}</h3>
              {competitions.map((competition) => (
                <CompetitionCard
                  key={competition.code}
                  competition={competition}
                />
              ))}
            </section>
          );
        })}
      </section>
    </>
  );
};

export default Competitions;

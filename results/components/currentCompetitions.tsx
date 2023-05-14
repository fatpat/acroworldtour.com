import classNames from "classnames";
import Link from "next/link";
import useSWR from "swr";

import Competition from "@/components/competition";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];

const CurrentEvents = () => {
  const { data: competitions, error } = useSWR<Competition[], Error>(
    "https://api-preprod.acroworldtour.com/public/competitions",
    fetcher
  );

  if (error) {
    return (
      <>
        <h2>Failed to load data ❌</h2>
        <small>
          The database may be down.
          <br />
          Check again in a few minutes.
        </small>
      </>
    );
  }
  if (!competitions)
    return (
      <>
        <h2>Loading...</h2>
      </>
    );
  const currentCompetitions = [competitions[0]];

  return (
    <article>
      {currentCompetitions.length ? (
        <h2>Current Event{currentCompetitions.length > 1 ? "s" : ""}</h2>
      ) : (
        <h2>
          {!currentCompetitions.length && "There is no event at the moment."}
        </h2>
      )}
      {currentCompetitions.map((competition) => Competition({ competition }))}
    </article>
  );
};

export default CurrentEvents;

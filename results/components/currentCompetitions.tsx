import useSWR from "swr";

import Competition from "@/components/competition";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

import FetchError from "./fetchError";
import FetchLoading from "./fetchLoading";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pluralH2 = "Current Events";
const singularH2 = "Current Event";
const noEventH2 = "There is no ongoing event.";

const CurrentEvents = () => {
  const { data: competitions, error } = useSWR<Competition[], Error>(
    "https://api-preprod.acroworldtour.com/public/competitions",
    fetcher
  );

  if (error)
    return (
      <section>
        <h2 className="mb-4 text-center">Couldn&apos;t fetch events.</h2>
        <FetchError />
      </section>
    );
  if (!competitions)
    return (
      <section>
        <h2 className="mb-4 text-center opacity-50">Fetching events...</h2>
        <FetchLoading />
      </section>
    );

  const currentCompetitions = [competitions[0]];

  const ongoing = currentCompetitions.length > 0;
  const isPlural = currentCompetitions.length > 1;

  return (
    <section>
      <h2 className="mb-4 text-center">
        {ongoing ? (isPlural ? pluralH2 : singularH2) : noEventH2}
      </h2>
      {currentCompetitions.map((competition) => Competition({ competition }))}
    </section>
  );
};

export default CurrentEvents;

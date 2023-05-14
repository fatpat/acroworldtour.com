import Link from "next/link";
import useSWR from "swr";

import Competition from "@/components/competition";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pageTitle = "Acro World Tour | Results";
const pageDescription =
  "Acro World Tour Results page. In this page you will find all seasons's results.";
const headerTitle = "Results";
const headerSubtitle = "Acro World Tour";

const Results = () => {
  const { data: competitions, error } = useSWR<Competition[]>(
    "https://api-preprod.acroworldtour.com/public/competitions",
    fetcher
  );

  if (error) return <div>Failed to load</div>;
  if (!competitions) return <div>Loading...</div>;

  const currentEvents = competitions.filter((comp) => comp.state === "open");
  const previousEvents = competitions.filter((comp) => comp.state === "closed");
  const scheduledEvents = competitions.filter((comp) => comp.state === "init");

  return (
    <>
      {currentEvents.length ? (
        <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
          {currentEvents.map((competition) => Competition({ competition }))}
        </section>
      ) : (
        <h2>No ongoing event.</h2>
      )}
      <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
        {previousEvents.map((competition) => Competition({ competition }))}
      </section>
      {scheduledEvents.length ? (
        <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
          {scheduledEvents.map((competition) => Competition({ competition }))}
        </section>
      ) : (
        <h2>No scheduled event.</h2>
      )}
    </>
  );
};

Results.pageTitle = pageTitle;
Results.pageDescription = pageDescription;
Results.headerTitle = headerTitle;
Results.headerSubtitle = headerSubtitle;

export default Results;

import Link from "next/link";
import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pageTitle = "Acro World Tour | Events";
const pageDescription =
  "Acro World Tour Events page. In this page you will find all the past, present, and scheduled competitions.";
const headerTitle = "Events";
const headerSubtitle = "Acro World Tour";

const Events = () => {
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
          {currentEvents.map((competition) => CompetitionCard({ competition }))}
        </section>
      ) : (
        <h2>No ongoing event.</h2>
      )}
      <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
        {previousEvents.map((competition) => CompetitionCard({ competition }))}
      </section>
      {scheduledEvents.length ? (
        <section className="mt-4 flex flex-wrap justify-start gap-x-8 gap-y-4">
          {scheduledEvents.map((competition) =>
            CompetitionCard({ competition })
          )}
        </section>
      ) : (
        <h2>No scheduled event.</h2>
      )}
    </>
  );
};

Events.pageTitle = pageTitle;
Events.pageDescription = pageDescription;
Events.headerTitle = headerTitle;
Events.headerSubtitle = headerSubtitle;

export default Events;

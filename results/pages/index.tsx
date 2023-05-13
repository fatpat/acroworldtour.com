import Link from "next/link";
import useSWR from "swr";

import About from "@/components/about";
import Competition from "@/components/competition";
import Download from "@/components/download";
import { components } from "@/interfaces/openapi";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pageTitle = "Acro World Tour | Home";
const pageDescription =
  "Home page for the official web application of the Acro World Tour. In this app you can find everything you need to know about competition results and your favourite pilots.";

const Home = () => {
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
  if (!competitions) return <h2>Fetching the latest data </h2>;

  // const currentCompetitions = competitions.filter((comp) => {
  //   comp.state === "open";
  // });

  const currentCompetitions = [competitions[0]];

  return (
    <>
      <About />
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
      <Download />
    </>
  );
};

Home.pageTitle = pageTitle;
Home.pageDescription = pageDescription;

export default Home;

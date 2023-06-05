import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import { API_URL } from "@/constants";
import { components } from "@/types";

import FetchError from "../ui/fetchError";
import FetchLoading from "../ui/fetchLoading";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pluralH2 = "Current Competitions";
const singularH2 = "Current Competition";
const noCompetitionH2 = "There are no ongoing competitions.";

const CurrentCompetitions = () => {
  const {
    data: competitions,
    error,
    isLoading,
  } = useSWR<Competition[], Error>(`${API_URL}/competitions/`);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!competitions) return <h2>Competitions not found</h2>;

  // const currentCompetitions = [...competitions]; // to test multiple competitions
  // const currentCompetitions = [competitions[0]]; // to test single competition
  // const currentCompetitions: Competition[] = []; // to test no competition

  const currentCompetitions = competitions.filter(
    (competition) => competition.state === "open"
  );

  const ongoing = currentCompetitions.length > 0;
  const isPlural = currentCompetitions.length > 1;

  return (
    <>
      <h2 className="mt-6">
        {ongoing ? (isPlural ? pluralH2 : singularH2) : noCompetitionH2}
      </h2>
      <section className="wrapper my-8">
        {currentCompetitions.map((competition) =>
          CompetitionCard({ competition })
        )}
      </section>
    </>
  );
};

export default CurrentCompetitions;

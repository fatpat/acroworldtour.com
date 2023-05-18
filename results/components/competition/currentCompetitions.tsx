import useSWR from "swr";

import CompetitionCard from "@/components/competition/competitionCard";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

import FetchError from "../ui/fetchError";
import FetchLoading from "../ui/fetchLoading";

type Competition = components["schemas"]["CompetitionPublicExport"];

const pluralH2 = "Current Events";
const singularH2 = "Current Event";
const noEventH2 = "There is no ongoing event.";

const CurrentCompetitions = () => {
  const { data: competitions, error } = useSWR<Competition[], Error>(
    `${API_URL}/public/competitions`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!competitions) return <FetchLoading />;

  // const currentCompetitions = [...competitions]; // to test multiple competitions
  const currentCompetitions = [competitions[0]]; // to test single competition
  // const currentCompetitions: Competition[] = []; // to test no competition

  // const currentCompetitions = competitions.filter(
  //   (competition) => competition.state === "open"
  // );

  const ongoing = currentCompetitions.length > 0;
  const isPlural = currentCompetitions.length > 1;

  return (
    <section>
      <h2 className="text-center">
        {ongoing ? (isPlural ? pluralH2 : singularH2) : noEventH2}
      </h2>
      <div>
        {currentCompetitions.map((competition) =>
          CompetitionCard({ competition })
        )}
      </div>
    </section>
  );
};

export default CurrentCompetitions;

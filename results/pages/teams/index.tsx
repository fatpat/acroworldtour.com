import useSWR from "swr";

import TeamCard from "@/components/team/teamCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Team = components["schemas"]["TeamExport"];

const pageTitle = "Acro World Tour | Teams";
const pageDescription = "All the teams of the Acro World Tour";

const Teams = () => {
  const { data: teams, error } = useSWR<Team[], Error>(
    `${API_URL}/teams`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!teams) return <FetchLoading />;

  return (
    <>
      <h2>All Teams</h2>
      <section className="mt-8 px-2">
        <div className="wrapper">
          {teams.map((team) => (
            <TeamCard key={team._id} team={team} />
          ))}
        </div>
      </section>
    </>
  );
};

Teams.pageTitle = pageTitle;
Teams.pageDescription = pageDescription;

export default Teams;

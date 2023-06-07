import { useEffect } from "react";
import useSWR, { preload } from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import TeamCard from "@/components/team/teamCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Team = components["schemas"]["TeamExport"];

preload(`${API_URL}/teams/`, fetcher);

const Teams = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Teams");
    setPageDescription("All the teams of the Acro World Tour");

    setHeaderTitle("Teams");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("teams");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const {
    data: teams,
    error,
    isLoading,
  } = useSWR<Team[], Error>(`${API_URL}/teams/`);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!teams) return <h2>No teams found.</h2>;

  return (
    <>
      <h2>All Teams</h2>
      <section className="wrapper mt-8">
        {teams.map((team) => (
          <TeamCard key={team._id} team={team} />
        ))}
      </section>
    </>
  );
};

export default Teams;

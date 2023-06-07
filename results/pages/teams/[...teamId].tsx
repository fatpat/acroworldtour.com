import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR, { preload } from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import TeamDetails from "@/components/team/teamDetails";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Team = components["schemas"]["TeamExport"];

const TeamPage = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();
  const router = useRouter();
  const [teamId, setTeamId] = useState("");

  useEffect(() => {
    if (router.isReady && router.query.teamId) {
      setTeamId(router.query.teamId[0]);
      preload(`${API_URL}/teams/${teamId}`, fetcher);
    }
  }, [router.isReady, router.query.teamId, teamId]);

  const {
    data: team,
    error,
    isLoading,
  } = useSWR<Team, Error>(`${API_URL}/teams/${teamId}`);

  useEffect(() => {
    if (team) {
      setPageTitle(team.name || "");
      setPageDescription(`Team page for ${team.name}`);
      setHeaderTitle(team.name || "");
      setHeaderSubtitle("Team");
      setActiveNav("teams");
    }
  }, [
    teamId,
    team,
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!team) return <h2>Team not found</h2>;

  return <TeamDetails team={team} />;
};

export default TeamPage;

import { useEffect } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import PilotCard from "@/components/pilot/pilotCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Team = components["schemas"]["TeamExport"];

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

  const { data: teams, error } = useSWR<Team[], Error>(
    `${API_URL}/teams`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!teams) return <FetchLoading />;

  return (
    <>
      <h2>All Teams</h2>
      <article className="grid grid-cols-12">
        <h4 className="col-span-4 col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
          Team
        </h4>
        <h4 className="col-span-8 col-start-5 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
          Pilots
        </h4>
        {teams
          .sort((a, b) => a.name.localeCompare(b.name))
          .map((team) => (
            <>
              <h5 className="col-span-4 col-start-1 border-[1px] py-2 pl-2 text-left">
                {team.name}
              </h5>
              <p className="col-span-8 col-start-5 border-[1px] py-2 pl-2 text-center">
                {/* TODO DEV: make <PilotCard> to be display side by side when enough space if available */}
                <PilotCard pilot={team.pilots[0]} />
                <PilotCard pilot={team.pilots[1]} />
              </p>
            </>
          ))}
      </article>
    </>
  );
};

export default Teams;

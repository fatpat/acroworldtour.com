import { useEffect } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import PilotCard from "@/components/pilot/pilotCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const Pilots = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Pilots");
    setPageDescription(`All the pilots of the Acro World Tour.`);
    setHeaderTitle("Pilots");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("pilots");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const { data: pilots, error } = useSWR<Pilot[], Error>(
    `${API_URL}/pilots`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!pilots) return <FetchLoading />;

  return (
    <>
      <h2>All Pilots</h2>
      <section className="mt-8 px-2">
        <div className="wrapper">
          {pilots.map((pilot) => (
            <PilotCard key={pilot.civlid} pilot={pilot} />
          ))}
        </div>
      </section>
    </>
  );
};

export default Pilots;

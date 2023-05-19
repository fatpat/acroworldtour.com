import useSWR from "swr";

import PilotCard from "@/components/pilot/pilotCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const pageTitle = "Acro World Tour | Pilots";
const pageDescription = "All the pilots of the Acro World Tour";

const Pilots = () => {
  const { data: pilots, error } = useSWR<Pilot[], Error>(
    `${API_URL}/pilots`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!pilots) return <FetchLoading />;

  return (
    <>
      <h2>All Pilots</h2>
      <section className="mt-8">
        <div className="wrapper">
          {pilots.map((pilot) => (
            <PilotCard key={pilot.civlid} pilot={pilot} />
          ))}
        </div>
      </section>
    </>
  );
};

Pilots.pageTitle = pageTitle;
Pilots.pageDescription = pageDescription;

export default Pilots;

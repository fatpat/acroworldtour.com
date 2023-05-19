import useSWR from "swr";

import PilotCard from "@/components/pilot/pilotCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const pageTitle = "Pilots";
const pageDescription = "All the pilots of the Acro World Tour";

const Pilots = () => {
  const { data: pilots, error } = useSWR<Pilot[], Error>(
    `${API_URL}/public/pilots`,
    fetcher
  );

  if (error) return <FetchError />;
  if (!pilots) return <FetchLoading />;

  return (
    <section>
      <h2 className="text-center">All Pilots</h2>
      <div>
        {pilots.map((pilot) => (
          <PilotCard key={pilot.civlid} pilot={pilot} />
        ))}
      </div>
    </section>
  );
};

Pilots.pageTitle = pageTitle;
Pilots.pageDescription = pageDescription;

export default Pilots;

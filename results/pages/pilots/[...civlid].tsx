import { useRouter } from "next/router";
import useSWR from "swr";

import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const pageTitle = "Acro World Tour | Pilots";
const pageDescription =
  "Acro World Tour Pilots page. In this page you will find all the pilots that are part of the Acro World Tour.";
const headerTitle = "Pilots";
const headerSubtitle = "Acro World Tour";

const Pilot = () => {
  const router = useRouter();
  const { id } = router.query;

  // const { data: pilot, error } = useSWR<Pilot>(
  //   `https://api-preprod.acroworldtour.com/public/pilots/${id}`,
  //   fetcher
  // );

  const { data: pilot, error } = useSWR<Pilot>(
    `https://api-preprod.acroworldtour.com/public/pilots/${id}`,
    fetcher
  );

  if (error) return <div>Failed to load</div>;
  if (!pilot) return <div>Loading...</div>;

  return (
    <main>
      <h1>PILOT {id}</h1>
      <h2>{pilot.name} fetched successfully</h2>
    </main>
  );
};

Pilot.pageTitle = pageTitle;
Pilot.pageDescription = pageDescription;
Pilot.headerTitle = headerTitle;
Pilot.headerSubtitle = headerSubtitle;

export default Pilot;

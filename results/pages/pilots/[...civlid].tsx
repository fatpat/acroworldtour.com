import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import PilotDetails from "@/components/pilot/pilotDetails";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Pilot = components["schemas"]["Pilot"];

const PilotPage = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();
  const router = useRouter();
  const [civlid, setCivlid] = useState("");

  useEffect(() => {
    if (router.isReady && router.query.civlid)
      setCivlid(router.query.civlid[0]);
  }, [router.isReady, router.query.civlid]);

  const { data: pilot, error } = useSWR<Pilot, Error>(
    civlid ? `${API_URL}/pilots/${civlid}` : null,
    fetcher
  );

  useEffect(() => {
    if (pilot) {
      setPageTitle(pilot.name || "");
      setPageDescription(`Pilot page for ${pilot.name}`);
      setHeaderTitle(pilot.name || "");
      setHeaderSubtitle(
        `${pilot.rank === 9999 ? "" : "#" + pilot.rank + " -"} ${civlid}` ||
          ""
      );
      setActiveNav("pilots");
    }
  }, [
    civlid,
    pilot,
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  if (error) return <FetchError />;
  if (!pilot) return <FetchLoading />;

  return <PilotDetails pilot={pilot} />;
};

export default PilotPage;

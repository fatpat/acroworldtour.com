import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR from "swr";

import PilotDetails from "@/components/pilot/pilotDetails";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

interface Props {
  pageTitle: string;
  pageDescription: string;
  headerTitle: string;
  headerSubtitle: string;
}

type Pilot = components["schemas"]["Pilot"];

const PilotPage = ({
  pageTitle,
  pageDescription,
  headerTitle,
  headerSubtitle,
}: Props) => {
  const router = useRouter();
  const [civlid, setCivlid] = useState("");

  useEffect(() => {
    if (router.isReady) setCivlid(router.query.civlid![0]);
  }, [router.isReady, router.query.civlid]);

  const { data: pilot, error } = useSWR<Pilot, Error>(
    civlid
      ? `https://api-preprod.acroworldtour.com/public/pilots/${civlid}`
      : null,
    fetcher
  );

  if (error) return <FetchError />;
  if (!pilot) return <FetchLoading />;

  return (
    <section className="mt-4 flex flex-row-reverse flex-wrap items-center justify-evenly gap-6">
      <PilotDetails pilot={pilot} />
    </section>
  );
};

export default PilotPage;

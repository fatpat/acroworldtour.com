import classNames from "classnames";
import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR from "swr";

import CompetitionDetails from "@/components/competition/competitionDetails";
import PilotDetails from "@/components/pilot/pilotDetails";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

interface Props {
  pageTitle: string;
  pageDescription: string;
  headerTitle: string;
  headerSubtitle: string;
}

type Competition = components["schemas"]["CompetitionPublicExportWithResults"];

const CompetitionPage = ({
  pageTitle,
  pageDescription,
  headerTitle,
  headerSubtitle,
}: Props) => {
  const router = useRouter();
  const [code, setCode] = useState<string>("");

  useEffect(() => {
    if (router.isReady && typeof router.query.code === "string")
      setCode(router.query.code);
  }, [router.isReady, router.query.code]);

  const { data: competition, error } = useSWR<Competition, Error>(
    code ? `${API_URL}/competitions/${code}` : null,
    fetcher
  );

  if (error) return <FetchError />;
  if (!competition) return <FetchLoading />;

  return <CompetitionDetails competition={competition} />;
};

CompetitionPage.pageDescription = "Pilot details";

export default CompetitionPage;

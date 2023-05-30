import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR from "swr";

import CompetitionDetails from "@/components/competition/competitionDetails";
import { useLayout } from "@/components/layout/layoutContext";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Competition = components["schemas"]["CompetitionPublicExportWithResults"];

const CompetitionPage = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

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

  useEffect(() => {
    if (competition) {
      setPageTitle(competition.name || "");
      setPageDescription(`Competition page for ${competition.name}`);
      setHeaderTitle(competition.name || "");
      setHeaderSubtitle(
        `${competition.type} - ${competition.location}` || ""
      );
      setActiveNav("competitions");
    }
  }, [
    competition,
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  if (error) return <FetchError />;
  if (!competition) return <FetchLoading />;

  return <CompetitionDetails competition={competition} />;
};

export default CompetitionPage;

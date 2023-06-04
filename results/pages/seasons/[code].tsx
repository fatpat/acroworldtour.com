import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import SeasonDetails from "@/components/season/seasonDetails";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

type Season = components["schemas"]["SeasonExport"];

const SeasonPage = () => {
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

  const {
    data: season,
    error,
    isLoading,
  } = useSWR<Season, Error>(
    code ? `${API_URL}/seasons/${code}` : null,
    fetcher
  );

  useEffect(() => {
    if (season) {
      setPageTitle(season.name || "");
      setPageDescription(`Season page for ${season.name}`);
      setHeaderTitle(season.name || "");
      setHeaderSubtitle(`${season.type} season` || "");
      setActiveNav("seasons");
    }
  }, [
    season,
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!season) return <h2>Season not found</h2>;

  return <SeasonDetails season={season} />;
};

export default SeasonPage;

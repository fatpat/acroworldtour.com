import { useEffect } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import TricksTable from "@/components/trick/tricksTable";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";

type Trick = components["schemas"]["Trick"];

const Tricks = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Tricks");
    setPageDescription("All the tricks of the Acro World Tour");

    setHeaderTitle("Tricks");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("tricks");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const {
    data: tricks,
    error,
    isLoading,
  } = useSWR<Trick[], Error>(`${API_URL}/tricks`);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!tricks) return <h2>Tricks not found</h2>;

  const solo_tricks = tricks.filter((t) => t.solo);
  const synchro_tricks = tricks.filter((t) => t.synchro && !t.solo);

  return (
    <>
      <h2>Solo and Synchro Tricks</h2>
      <TricksTable tricks={solo_tricks} className="grid grid-cols-12" />
      <h2>Synchro only Tricks</h2>
      <TricksTable tricks={synchro_tricks} className="grid grid-cols-12" />
    </>
  );
};

export default Tricks;

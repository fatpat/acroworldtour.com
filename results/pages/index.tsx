import dynamic from "next/dynamic";
import { useEffect } from "react";

import CurrentCompetitions from "@/components/competition/currentCompetitions";
import Download from "@/components/download";
import { useLayout } from "@/components/layout/layoutContext";

const About = dynamic(() => import("@/components/about"), {
  ssr: false,
});

const Home = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Home");
    setPageDescription(
      `Home page for the official web application of the Acro World Tour.
      In this app you can find everything you need to know about competition results and your favourite pilots.`
    );
    setHeaderTitle("");
    setHeaderSubtitle("");
    setActiveNav("home");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  return (
    <>
      <About />
      <CurrentCompetitions />
      <Download />
    </>
  );
};

export default Home;

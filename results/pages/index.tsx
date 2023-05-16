import dynamic from "next/dynamic";

import CurrentCompetitions from "@/components/currentCompetitions";
import Download from "@/components/download";

const pageTitle = "Acro World Tour | Home";
const pageDescription =
  "Home page for the official web application of the Acro World Tour. In this app you can find everything you need to know about competition results and your favourite pilots.";

const About = dynamic(() => import("@/components/about"), {
  ssr: false,
});

const Home = () => (
  <>
    <About />
    <CurrentCompetitions />
    <Download />
  </>
);

Home.pageTitle = pageTitle;
Home.pageDescription = pageDescription;

export default Home;

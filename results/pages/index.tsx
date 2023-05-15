import classNames from "classnames";
import dynamic from "next/dynamic";

import Download from "@/components/download";

const pageTitle = "Acro World Tour | Home";
const pageDescription =
  "Home page for the official web application of the Acro World Tour. In this app you can find everything you need to know about competition results and your favourite pilots.";

const About = dynamic(() => import("@/components/about"), {
  ssr: false,
} );

const Home = () => {
  // const currentCompetitions = competitions.filter((comp) => {
  //   comp.state === "open";
  // });

  return (
    <>
      <About />
      <section className="flex flex-col gap-8">
        <article
          className={classNames(
            "my-8 flex h-24 w-full max-w-lg flex-col items-center justify-center rounded-xl bg-awtgrey-100 shadow-md"
          )}
        >
          <h2>Loading...</h2>
          {/* <Image
            src="/images/acro-world-tour-logo.png"
            alt=""
            width="0"
            height="0"
            className=""
          /> */}
        </article>

        <Download />
      </section>
    </>
  );
};

Home.pageTitle = pageTitle;
Home.pageDescription = pageDescription;

export default Home;

import classNames from "classnames";

import About from "@/components/about";
import Download from "@/components/download";

const pageTitle = "Acro World Tour | Home";
const pageDescription =
  "Home page for the official web application of the Acro World Tour. In this app you can find everything you need to know about competition results and your favourite pilots.";

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
            "flex h-24 w-full max-w-lg items-center justify-center rounded-xl bg-gray-100 shadow-md my-8"
          )}
        >
          <h2>Loading...</h2>
        </article>

        <Download />
      </section>
    </>
  );
};

Home.pageTitle = pageTitle;
Home.pageDescription = pageDescription;

export default Home;

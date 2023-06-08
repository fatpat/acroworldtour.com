import { Fragment, useEffect } from "react";
import useSWR from "swr";

import JudgeCard from "@/components/judge/judgeCard";
import { useLayout } from "@/components/layout/layoutContext";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";

type Judge = components["schemas"]["Judge"];

const Judges = () => {
  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  useEffect(() => {
    setPageTitle("Acro World Tour | Judges");
    setPageDescription("All the judges of the Acro World Tour");

    setHeaderTitle("Judges");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("judges");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  const {
    data: judges,
    error,
    isLoading,
  } = useSWR<Judge[], Error>(`${API_URL}/judges/`);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!judges) return <h2>Judges not found</h2>;

  const filterAndSortJudges = (judges: Judge[], level: Judge["level"]) =>
    judges
      .filter((judge) => judge.level === level)
      .sort((a, b) => a.name.localeCompare(b.name));

  const ratedJudges = [
    { judges: filterAndSortJudges(judges, "senior"), category: "Senior" },
    { judges: filterAndSortJudges(judges, "certified"), category: "Certified" },
    { judges: filterAndSortJudges(judges, "trainee"), category: "Trainee" },
  ];

  return (
    <>
      {ratedJudges.map((rating, index) => (
        <Fragment key={rating.category}>
          <h2>{rating.category} Judges</h2>
          <section className="my-8 flex flex-wrap justify-center gap-8">
            {rating.judges.map((judge) => (
              <JudgeCard key={judge._id} judge={judge} />
            ))}
          </section>
          {index < ratedJudges.length - 1 && <hr className="mb-4" />}
        </Fragment>
      ))}
    </>
  );
};

export default Judges;

import { useEffect } from "react";
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
  } = useSWR<Judge[], Error>(`${API_URL}/judges`);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!judges) return <h2>Judges not found</h2>;

  const seniors = judges
    .filter((j) => j.level == "senior")
    .sort((a, b) => a.name.localeCompare(b.name));
  const certifieds = judges
    .filter((j) => j.level == "certified")
    .sort((a, b) => a.name.localeCompare(b.name));
  const trainees = judges
    .filter((j) => j.level == "trainee")
    .sort((a, b) => a.name.localeCompare(b.name));

  return (
    <>
      <h2>Senior Judges</h2>
      <section className="mt-8 px-2">
        <div className="wrapper">
          {seniors.map((judge) => (
            <JudgeCard key={judge._id} judge={judge} />
          ))}
        </div>
      </section>
      <h2>Certified Judges</h2>
      <section className="mt-8 px-2">
        <div className="wrapper">
          {certifieds.map((judge) => (
            <JudgeCard key={judge._id} judge={judge} />
          ))}
        </div>
      </section>
      <h2>Trainee Judges</h2>
      <section className="mt-8 px-2">
        <div className="wrapper">
          {trainees.map((judge) => (
            <JudgeCard key={judge._id} judge={judge} />
          ))}
        </div>
      </section>
    </>
  );
};

export default Judges;

import classNames from "classnames";
import { ChangeEvent, useEffect, useState } from "react";
import useSWR from "swr";

import { useLayout } from "@/components/layout/layoutContext";
import SeasonCard from "@/components/season/seasonCard";
import FetchError from "@/components/ui/fetchError";
import FetchLoading from "@/components/ui/fetchLoading";
import { API_URL } from "@/constants";
import { components } from "@/types";

type Season = components["schemas"]["SeasonExport"];

const currentYear = new Date().getFullYear();

const Seasons = () => {
  const [selectedYear, setSelectedYear] = useState(currentYear);

  const {
    setPageTitle,
    setPageDescription,
    setHeaderTitle,
    setHeaderSubtitle,
    setActiveNav,
  } = useLayout();

  const {
    data: seasons,
    error,
    isLoading,
  } = useSWR<Season[]>(`${API_URL}/seasons`);

  useEffect(() => {
    setPageTitle("Acro World Tour | Seasons");
    setPageDescription(`All the seasons of the Acro World Tour.`);
    setHeaderTitle("Seasons");
    setHeaderSubtitle("Acro World Tour");
    setActiveNav("seasons");
  }, [
    setActiveNav,
    setHeaderSubtitle,
    setHeaderTitle,
    setPageDescription,
    setPageTitle,
  ]);

  if (isLoading) return <FetchLoading />;
  if (error) return <FetchError />;
  if (!seasons) return <h2>Seasons not found</h2>;

  const filteredSeasons = seasons.filter(
    (season) => season.year === selectedYear
  );

  const soloSeasons = filteredSeasons.filter(
    (season) => season.type === "solo"
  );

  soloSeasons.sort(
    (a, b) =>
      b.year - a.year ||
      (a.index || 999) - (b.index || 999) ||
      a.name.localeCompare(b.name)
  );

  const years = [
    ...new Set(seasons.map((season) => Number(season.year))).add(currentYear),
  ].sort((a, b) => b - a);

  const YearSelector = ({ years }: { years: number[] }) => {
    const handleYearChange = ({ target }: ChangeEvent<HTMLSelectElement>) => {
      const year = parseInt(target.value);
      setSelectedYear(year);
    };
    return (
      <header className="font-semibold opacity-95">
        <h2 className="-mt-2 mb-6 flex items-baseline">
          <label htmlFor="year-selector">
            {filteredSeasons.length === 0 ? `No seasons in ` : `Seasons in `}
          </label>
          <select
            id="year-selector"
            title="Select a year"
            value={selectedYear ?? ""}
            className="border-0 font-sans text-lg sm:text-xl"
            onChange={handleYearChange}
          >
            {years.map((year) => (
              <option
                key={year}
                value={year}
                className="text-base text-awt-dark-600"
              >
                {year}
              </option>
            ))}
          </select>
        </h2>
      </header>
    );
  };
  return (
    <>
      <YearSelector years={years} />
      <section className={classNames("wrapper mb-8")}>
        {soloSeasons.map((season) => (
          <SeasonCard key={season.code} season={season} />
        ))}
      </section>
    </>
  );
};

export default Seasons;

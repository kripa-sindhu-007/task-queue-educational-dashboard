import Hero from "@/components/landing/Hero";
import ValueProps from "@/components/landing/ValueProps";
import FeatureGrid from "@/components/landing/FeatureGrid";
import EntryCards from "@/components/landing/EntryCards";
import LandingFooter from "@/components/landing/LandingFooter";

export default function LandingPage() {
  return (
    <main>
      <Hero />
      <ValueProps />
      <FeatureGrid />
      <EntryCards />
      <LandingFooter />
    </main>
  );
}

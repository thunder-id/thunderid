/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useThunderID } from "@thunderid/react";
import {
  ArrowRight,
  Clock3,
  Hotel,
  LifeBuoy,
  Plane,
  Search,
  ShieldCheck,
  Sparkles,
  Star
} from "lucide-react";
import { SearchPanel } from "../components/SearchPanel";

const pageDetails = {
  flights: {
    heroHeading: "Find flights for wherever you are headed.",
    heroCopy:
      "Compare flexible fares, departure times, and routes while keeping bookings connected to a secure account experience.",
    freshPicksHeading: "Flight ideas for your next window of free time.",
    whyTitle: "A cleaner path from route search to booking.",
    whyCopy:
      "Keep the practical pieces close together: route ideas, dates, traveler counts, and authenticated bookings.",
    highlights: [
      {
        icon: <Sparkles size={22} />,
        title: "Flexible fares",
        copy: "Compare routes, cabins, and timings from one calm workspace."
      },
      {
        icon: <ShieldCheck size={22} />,
        title: "Protected bookings",
        copy: "Keep booked trips connected to secure account sign-in."
      },
      {
        icon: <Clock3 size={22} />,
        title: "Fast comparisons",
        copy: "Filter practical options for routes, dates, and travelers."
      }
    ],
    deals: [
      {
        className: "deal-card--mint",
        icon: <Plane size={22} />,
        route: "Colombo to Singapore",
        title: "Nonstop city break with smart baggage picks",
        price: "$420",
        meta: "Jun 12 - Jun 18"
      },
      {
        className: "deal-card--coral",
        icon: <Plane size={22} />,
        route: "Colombo to Tokyo",
        title: "Evening departure with a comfortable connection",
        price: "$610",
        meta: "Economy round trip"
      },
      {
        className: "deal-card--gold",
        icon: <Plane size={22} />,
        route: "Colombo to Dubai",
        title: "Short-haul escape with flexible return dates",
        price: "$380",
        meta: "Best value this week"
      }
    ]
  },
  hotels: {
    heroHeading: "Find stays that fit the way you travel.",
    heroCopy:
      "Compare areas, nightly rates, and useful amenities before choosing the stay that keeps your trip simple.",
    freshPicksHeading: "Stay ideas close to food, rail, and plans.",
    whyTitle: "A cleaner path from area search to reservation.",
    whyCopy:
      "Keep destinations, dates, guest counts, and stay preferences in one focused search flow.",
    highlights: [
      {
        icon: <Hotel size={22} />,
        title: "Area-first search",
        copy: "Start from the neighborhood or district that fits the trip."
      },
      {
        icon: <Star size={22} />,
        title: "Useful amenities",
        copy: "Compare the details that matter for repeatable travel decisions."
      },
      {
        icon: <Clock3 size={22} />,
        title: "Quick scanning",
        copy: "Nightly rates and dates stay easy to compare."
      }
    ],
    deals: [
      {
        className: "deal-card--mint",
        icon: <Hotel size={22} />,
        route: "Singapore Marina",
        title: "Waterfront stays close to transit and evening plans",
        price: "$210",
        meta: "Average nightly rate"
      },
      {
        className: "deal-card--coral",
        icon: <Hotel size={22} />,
        route: "Tokyo Shibuya",
        title: "Walkable rooms near food, rail, and late-night plans",
        price: "$186",
        meta: "Guest favorite area"
      },
      {
        className: "deal-card--gold",
        icon: <Hotel size={22} />,
        route: "London Kings Cross",
        title: "Rail-friendly stays for a compact city visit",
        price: "$240",
        meta: "Central location"
      }
    ]
  },
  trips: {
    heroHeading: "Find trip ideas when the destination is still forming.",
    heroCopy:
      "Explore city plans, estimated budgets, and flexible destination ideas before turning curiosity into a booking.",
    freshPicksHeading: "Trip ideas with enough structure to start.",
    whyTitle: "A cleaner path from inspiration to itinerary.",
    whyCopy:
      "Keep destinations, estimates, and plan types together so comparing possible trips feels lighter.",
    highlights: [
      {
        icon: <Sparkles size={22} />,
        title: "Curated ideas",
        copy: "Compare ready-to-shape city plans before you commit."
      },
      {
        icon: <Star size={22} />,
        title: "Budget signals",
        copy: "Estimated trip totals keep options grounded."
      },
      {
        icon: <LifeBuoy size={22} />,
        title: "Flexible planning",
        copy: "Move from inspiration to search without switching tools."
      }
    ],
    deals: [
      {
        className: "deal-card--mint",
        icon: <Sparkles size={22} />,
        route: "Singapore highlights",
        title: "A food, gardens, and skyline plan for a long weekend",
        price: "$620",
        meta: "Estimated trip total"
      },
      {
        className: "deal-card--coral",
        icon: <Sparkles size={22} />,
        route: "Tokyo first-timer",
        title: "A practical city plan with rail-friendly neighborhoods",
        price: "$920",
        meta: "Five-day estimate"
      },
      {
        className: "deal-card--gold",
        icon: <Sparkles size={22} />,
        route: "Dubai highlights",
        title: "A polished three-day plan with room to wander",
        price: "$780",
        meta: "Estimated trip total"
      }
    ]
  }
};

export function getDisplayName(user) {
  if (!user) return "";
  const given = user.given_name || user.name?.givenName || "";
  const family = user.family_name || user.name?.familyName || "";
  const full = `${given} ${family}`.trim();
  if (full) return full;
  if (typeof user.name === "string" && user.name.trim()) return user.name.trim();
  return (
    user.preferred_username ||
    user.username ||
    user.userName ||
    user.email ||
    user.mail ||
    ""
  );
}

export function HomePage({
  category = "flights",
  hideHeroSupport = false,
  heroHeading,
  locations,
  onSearch,
  showSearch = true
}) {
  const details = pageDetails[category] || pageDetails.flights;
  const isGreetingHero = hideHeroSupport;

  const faqs = [
    {
      question: "How does Skyscanner work?",
      answer: "We’re a flight, car hire and hotel search engine. We scan all the top airlines and travel providers across the net, so you can compare flight fares and other travel costs in one place. Once you’ve found the best flight, car hire or hotel, you book directly with the provider."
    },
    {
      question: "How can I find the cheapest flight using Skyscanner?",
      answer: "Finding flights is easy here – over 100 million savvy travellers come to us each month to find cheap flight tickets, hotels and car hire. Here are a few simple tips on how to get the most out of your flight search. Save money and time. Whether it's the fastest flight or the smartest stay, you can pick your preferred flight provider or hotel based on real traveller ratings, and book instantly. Search Everywhere. Go anywhere. Fancy a trip but don’t mind where? Or perhaps you want to discover somewhere new. Search ‘Everywhere’ for the best budget airfare anywhere on any given day. Find the cheapest time to fly. If you have a destination in mind and want to find the cheapest flight, choose ‘Cheapest month’ when you search. Then find flights on the cheapest day."
    },
    {
      question: "Where should I book a flight to right now?",
      answer: "If you're looking for inspiration for your next trip, search Everywhere to find a cheap flight ticket anywhere."
    },
    {
      question: "Do I book my flight with Skyscanner?",
      answer: "We’re a search engine, so after you’ve found the best flight ticket you’ll book directly with the airline or travel provider on their site. This will give you the opportunity to add any loyalty information you would like and select your preferred flight options, such as seating."
    },
    {
      question: "What happens after I have booked my flight?",
      answer: "Your flight booking confirmation email and all the other info you'll need will come from the airline or provider you booked with. If you have any follow-up questions about your booking, or want to change or cancel your flight, you’d need to get in touch with them."
    },
    {
      question: "Does Skyscanner do hotels too?",
      answer: "Yes. You can use the same magic that powers flight search to find your perfect stay anywhere."
    },
    {
      question: "What about car hire?",
      answer: "You can also use Skyscanner to search for and compare cheap car hire in seconds, then pick up your wheels from the airport as soon as you touch down."
    },
    {
      question: "What’s a Price Alert?",
      answer: "If you set up a Price Alert, we’ll watch the price of plane tickets for you, and let you know via email or push notification via the app if they rise or fall."
    }
  ];

  return (
    <main>
      <section className={`hero ${isGreetingHero ? "hero--greeting" : ""}`} id="search">
        <div className="hero-copy">
          <h1>{heroHeading || details.heroHeading}</h1>
          {!hideHeroSupport && (
            <>
              <p>{details.heroCopy}</p>
              <div className="hero-actions" aria-label="Popular planning links">
                <a className="secondary-button" href="#deals">
                  <Sparkles size={18} />
                  See ideas
                </a>
                <a className="link-button" href="#faq">
                  FAQ
                  <ArrowRight size={18} />
                </a>
              </div>
            </>
          )}
        </div>
        {showSearch && (
          <SearchPanel
            compact
            defaultCategory={category}
            locations={locations}
            onSearch={onSearch}
          />
        )}
      </section>

      <section className="insight-strip" aria-label="Wayfinder highlights">
        {details.highlights.map((item) => (
          <div key={item.title}>
            {item.icon}
            <span>
              <strong>{item.title}</strong>
              <small>{item.copy}</small>
            </span>
          </div>
        ))}
      </section>

      <section className="content-band" id="deals">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Fresh picks</p>
            <h2>{details.freshPicksHeading}</h2>
          </div>
          <a className="link-button" href="#search">
            Start searching
            <ArrowRight size={18} />
          </a>
        </div>
        <div className="deal-grid">
          {details.deals.map((deal) => (
            <article className={`deal-card ${deal.className}`} key={deal.title}>
              <div className="deal-icon">{deal.icon}</div>
              <p>{deal.route}</p>
              <h3>{deal.title}</h3>
              <span>{deal.meta}</span>
              <strong>{deal.price}</strong>
              <button className="card-action" type="button" onClick={() => window.location.hash = "search"}>
                Explore
              </button>
            </article>
          ))}
        </div>
      </section>

      <section className="two-column content-band">
        <div>
          <p className="eyebrow">Why Wayfinder</p>
          <h2>{details.whyTitle}</h2>
          <p className="section-copy">{details.whyCopy}</p>
          <a className="secondary-button" href="#search">
            <Search size={18} />
            Plan a route
          </a>
        </div>
        <div className="stay-list">
          <article className="stay-card">
            <div>
              <h3>Secure account journey</h3>
              <p>Sign in, sign up, and booking actions sit naturally in the flow.</p>
            </div>
            <div className="stay-meta">
              <span>
                <ShieldCheck size={18} />
              </span>
              <strong>Protected</strong>
            </div>
          </article>
          <article className="stay-card">
            <div>
              <h3>Human-friendly choices</h3>
              <p>Flights, hotels, and trip ideas are grouped for repeat comparison.</p>
            </div>
            <div className="stay-meta">
              <span>
                <Star size={18} />
              </span>
              <strong>Curated</strong>
            </div>
          </article>
          <article className="stay-card">
            <div>
              <h3>Support-ready setup</h3>
              <p>Fallback sample data keeps the front end useful while APIs connect.</p>
            </div>
            <div className="stay-meta">
              <span>
                <LifeBuoy size={18} />
              </span>
              <strong>Resilient</strong>
            </div>
          </article>
        </div>
      </section>

      <section className="faq-section" id="faq">
        <div className="section-heading">
          <div>
            <p className="eyebrow">FAQ</p>
            <h2>Answers before takeoff.</h2>
          </div>
        </div>
        <div className="faq-grid">
          {faqs.map((faq) => (
            <details className="faq-item" key={faq.question}>
              <summary>{faq.question}</summary>
              <p>{faq.answer}</p>
            </details>
          ))}
        </div>
      </section>
    </main>
  );
}

export function SignedInHomePage({ category = "flights", locations, onSearch }) {
  const { isSignedIn, user } = useThunderID();

  if (!isSignedIn) {
    return <HomePage category={category} locations={locations} onSearch={onSearch} />;
  }

  const greetingName = getDisplayName(user) || "Traveler";

  return (
    <HomePage
      hideHeroSupport
      category={category}
      heroHeading={`Welcome back, ${greetingName}.`}
      locations={locations}
      onSearch={onSearch}
      showSearch
    />
  );
}

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

import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  CalendarDays,
  Check,
  ChevronLeft,
  ChevronRight,
  Hotel,
  MapPin,
  Minus,
  Plane,
  Plus,
  Search,
  Sparkles,
  UsersRound
} from "lucide-react";

const categoryPaths = {
  flights: "/flights",
  hotels: "/hotels",
  trips: "/trips"
};

function LocationField({
  icon,
  label,
  name,
  defaultValue,
  locations,
  placeholder,
  isOpen,
  onOpen,
  onClose
}) {
  const [value, setValue] = useState(defaultValue || "");
  const normalizedValue = value.trim().toLowerCase();
  const filteredLocations = locations
    .filter((location) => location.name.toLowerCase().includes(normalizedValue))
    .slice(0, 8);

  useEffect(() => {
    setValue(defaultValue || "");
  }, [defaultValue]);

  function selectLocation(locationName) {
    setValue(locationName);
    onClose();
  }

  return (
    <label className="field field--wide location-field">
      <span>{label}</span>
      <div className="field-control">
        {icon}
        <input
          autoComplete="off"
          name={name}
          value={value}
          placeholder={placeholder}
          aria-label={label}
          onChange={(event) => {
            setValue(event.target.value);
            onOpen();
          }}
          onClick={onOpen}
          onFocus={onOpen}
        />
      </div>
      {isOpen && filteredLocations.length > 0 && (
        <div className="location-menu" role="listbox">
          {filteredLocations.map((location) => (
            <button
              className="location-option"
              key={`${location.type}-${location.name}`}
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectLocation(location.name)}
            >
              <MapPin size={16} />
              <span>{location.name}</span>
              <small>{location.type}</small>
              {location.name === value && <Check size={16} />}
            </button>
          ))}
        </div>
      )}
    </label>
  );
}

const monthFormatter = new Intl.DateTimeFormat("en", { month: "long" });
const shortDateFormatter = new Intl.DateTimeFormat("en", { month: "short", day: "numeric" });

function addMonths(date, months) {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

function dateKey(date) {
  return date.toISOString().slice(0, 10);
}

function formatDateRange(startDate, endDate) {
  if (!startDate || !endDate) {
    return "Select dates";
  }

  return `${shortDateFormatter.format(startDate)} - ${shortDateFormatter.format(endDate)}`;
}

function buildCalendarDays(monthDate) {
  const year = monthDate.getFullYear();
  const month = monthDate.getMonth();
  const firstDay = new Date(year, month, 1);
  const lastDay = new Date(year, month + 1, 0);
  const leadingDays = (firstDay.getDay() + 6) % 7;
  const days = [];

  for (let index = 0; index < leadingDays; index += 1) {
    days.push(null);
  }

  for (let day = 1; day <= lastDay.getDate(); day += 1) {
    days.push(new Date(year, month, day));
  }

  return days;
}

function DateField({ defaultValue, isOpen, onOpen, onClose }) {
  const [visibleMonth, setVisibleMonth] = useState(new Date(2026, 4, 1));
  const [startDate, setStartDate] = useState(new Date(2026, 5, 12));
  const [endDate, setEndDate] = useState(new Date(2026, 5, 18));
  const displayValue = formatDateRange(startDate, endDate) || defaultValue;

  function selectDate(date) {
    if (!startDate || (startDate && endDate) || date < startDate) {
      setStartDate(date);
      setEndDate(null);
      return;
    }

    setEndDate(date);
  }

  function renderMonth(monthDate) {
    const days = buildCalendarDays(monthDate);

    return (
      <div className="calendar-month">
        <h3>{monthFormatter.format(monthDate)}</h3>
        <div className="calendar-weekdays">
          {["M", "T", "W", "T", "F", "S", "S"].map((day, index) => (
            <span key={`${day}-${index}`}>{day}</span>
          ))}
        </div>
        <div className="calendar-grid">
          {days.map((date, index) => {
            if (!date) {
              return <span className="calendar-empty" key={`empty-${index}`} />;
            }

            const isStart = startDate && dateKey(date) === dateKey(startDate);
            const isEnd = endDate && dateKey(date) === dateKey(endDate);
            const isInRange = startDate && endDate && date > startDate && date < endDate;

            return (
              <button
                className={`calendar-day ${isStart || isEnd ? "calendar-day--selected" : ""} ${
                  isInRange ? "calendar-day--range" : ""
                }`}
                key={dateKey(date)}
                type="button"
                onClick={() => selectDate(date)}
              >
                {date.getDate()}
              </button>
            );
          })}
        </div>
      </div>
    );
  }

  return (
    <label className="field date-field">
      <span>Dates</span>
      <div className="field-control">
        <CalendarDays size={18} />
        <input
          name="dates"
          readOnly
          value={displayValue}
          aria-label="Travel dates"
          onFocus={onOpen}
          onClick={onOpen}
        />
      </div>
      {isOpen && (
        <div className="date-menu">
          <div className="calendar-wrap">
            <button
              className="calendar-nav"
              type="button"
              onClick={() => setVisibleMonth(addMonths(visibleMonth, -1))}
              aria-label="Previous month"
            >
              <ChevronLeft size={26} />
            </button>
            {renderMonth(visibleMonth)}
            {renderMonth(addMonths(visibleMonth, 1))}
            <button
              className="calendar-nav"
              type="button"
              onClick={() => setVisibleMonth(addMonths(visibleMonth, 1))}
              aria-label="Next month"
            >
              <ChevronRight size={26} />
            </button>
          </div>
          <div className="date-menu-footer">
            <button className="apply-button" type="button" onClick={onClose}>
              Apply
            </button>
          </div>
        </div>
      )}
    </label>
  );
}

function TravelersField({ defaultValue, isOpen, onOpen, onClose }) {
  const [adults, setAdults] = useState(2);
  const [children, setChildren] = useState(0);
  const displayValue =
    `${adults} adult${adults === 1 ? "" : "s"}${children > 0 ? `, ${children} child${children === 1 ? "" : "ren"}` : ""}` ||
    defaultValue;

  function changeAdults(change) {
    setAdults((current) => Math.max(1, current + change));
  }

  function changeChildren(change) {
    setChildren((current) => Math.max(0, current + change));
  }

  return (
    <label className="field travelers-field">
      <span>Travelers</span>
      <div className="field-control">
        <UsersRound size={18} />
        <input
          name="travelers"
          readOnly
          value={displayValue}
          aria-label="Travelers"
          onFocus={onOpen}
          onClick={onOpen}
        />
      </div>
      {isOpen && (
        <div className="travelers-menu">
          <div className="traveler-row">
            <div>
              <strong>Adults</strong>
              <span>Aged 18+</span>
            </div>
            <div className="stepper">
              <button type="button" disabled={adults <= 1} onClick={() => changeAdults(-1)}>
                <Minus size={20} />
              </button>
              <strong>{adults}</strong>
              <button type="button" onClick={() => changeAdults(1)}>
                <Plus size={22} />
              </button>
            </div>
          </div>
          <div className="traveler-row">
            <div>
              <strong>Children</strong>
              <span>Aged 0 to 17</span>
            </div>
            <div className="stepper">
              <button type="button" disabled={children <= 0} onClick={() => changeChildren(-1)}>
                <Minus size={20} />
              </button>
              <strong>{children}</strong>
              <button type="button" onClick={() => changeChildren(1)}>
                <Plus size={22} />
              </button>
            </div>
          </div>
          <p>
            Your age at time of travel must be valid for the age category booked.
            Airlines have restrictions on under 18s travelling alone.
          </p>
          <p>
            Age limits and policies for travelling with children may vary so please check
            with the airline before booking.
          </p>
          <button className="apply-button apply-button--full" type="button" onClick={onClose}>
            Apply
          </button>
        </div>
      )}
    </label>
  );
}

export function SearchPanel({
  compact = false,
  defaultCategory = "flights",
  initialCriteria,
  locations,
  navigateOnCategoryChange = true,
  onSearch
}) {
  const navigate = useNavigate();
  const [category, setCategory] = useState(initialCriteria?.category || defaultCategory);
  const [tripType, setTripType] = useState(initialCriteria?.tripType || "round-trip");
  const [openDropdown, setOpenDropdown] = useState(null);
  const panelRef = useRef(null);

  useEffect(() => {
    setCategory(initialCriteria?.category || defaultCategory);
    setTripType(initialCriteria?.tripType || "round-trip");
  }, [defaultCategory, initialCriteria?.category, initialCriteria?.tripType]);

  useEffect(() => {
    function handlePointerDown(event) {
      if (panelRef.current && !panelRef.current.contains(event.target)) {
        setOpenDropdown(null);
      }
    }

    document.addEventListener("pointerdown", handlePointerDown);

    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, []);

  function handleSubmit(event) {
    event.preventDefault();

    const formData = new FormData(event.currentTarget);

    onSearch({
      category,
      tripType: category === "flights" ? tripType : "",
      from: formData.get("from"),
      to: formData.get("to"),
      dates: formData.get("dates"),
      travelers: formData.get("travelers")
    });
  }

  function selectCategory(nextCategory) {
    setCategory(nextCategory);
    setOpenDropdown(null);

    if (navigateOnCategoryChange) {
      navigate(categoryPaths[nextCategory]);
    }
  }

  const isHotelSearch = category === "hotels";
  const destinationPlaceholder = isHotelSearch ? "Choose area" : "Choose destination";

  return (
    <section
      className={`search-panel ${compact ? "search-panel--compact" : ""}`}
      ref={panelRef}
      aria-label="Search travel"
    >
      <div className="tabs" aria-label="Booking type">
        <button
          className={`tab ${category === "flights" ? "tab--active" : ""}`}
          type="button"
          onClick={() => selectCategory("flights")}
        >
          <Plane size={18} />
          Flights
        </button>
        <button
          className={`tab ${category === "hotels" ? "tab--active" : ""}`}
          type="button"
          onClick={() => selectCategory("hotels")}
        >
          <Hotel size={18} />
          Hotels
        </button>
        <button
          className={`tab ${category === "trips" ? "tab--active" : ""}`}
          type="button"
          onClick={() => selectCategory("trips")}
        >
          <Sparkles size={18} />
          Trips
        </button>
      </div>

      {category === "flights" && (
        <div className="trip-type">
          <button
            className={`pill ${tripType === "round-trip" ? "pill--selected" : ""}`}
            type="button"
            onClick={() => setTripType("round-trip")}
          >
            Round trip
          </button>
          <button
            className={`pill ${tripType === "one-way" ? "pill--selected" : ""}`}
            type="button"
            onClick={() => setTripType("one-way")}
          >
            One way
          </button>
          <button
            className={`pill ${tripType === "multi-city" ? "pill--selected" : ""}`}
            type="button"
            onClick={() => setTripType("multi-city")}
          >
            Multi-city
          </button>
        </div>
      )}

      <form className={`search-grid search-grid--${category}`} onSubmit={handleSubmit}>
        {!isHotelSearch && (
          <LocationField
            defaultValue={initialCriteria?.from || "Colombo"}
            icon={<MapPin size={18} />}
            isOpen={openDropdown === "from"}
            label="From"
            locations={locations[category] || []}
            name="from"
            onClose={() => setOpenDropdown(null)}
            onOpen={() => setOpenDropdown("from")}
            placeholder="Choose origin"
          />
        )}
        <LocationField
          defaultValue={initialCriteria?.to || "Singapore"}
          icon={isHotelSearch ? <MapPin size={18} /> : <Plane size={18} />}
          isOpen={openDropdown === "to"}
          label={isHotelSearch ? "Destination" : "To"}
          locations={locations[category] || []}
          name="to"
          onClose={() => setOpenDropdown(null)}
          onOpen={() => setOpenDropdown("to")}
          placeholder={destinationPlaceholder}
        />
        <DateField
          defaultValue={initialCriteria?.dates}
          isOpen={openDropdown === "dates"}
          onClose={() => setOpenDropdown(null)}
          onOpen={() => setOpenDropdown("dates")}
        />
        <TravelersField
          defaultValue={initialCriteria?.travelers}
          isOpen={openDropdown === "travelers"}
          onClose={() => setOpenDropdown(null)}
          onOpen={() => setOpenDropdown("travelers")}
        />
        <button className="search-button" type="submit">
          <Search size={20} />
          Search
        </button>
      </form>
    </section>
  );
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Flight } from './travel';

/** The flights every company can search. Fares cover each outcome of the default travel policy. */
const FLIGHTS: Flight[] = [
  { id: 'WF101', from: 'Colombo', to: 'Singapore', airline: 'SkyBridge', departure: '08:15', arrival: '14:40', duration: '3h 55m', stops: 0, price: 268, cabin: 'Economy' },
  { id: 'WF102', from: 'Colombo', to: 'Singapore', airline: 'Lanka Air', departure: '13:05', arrival: '19:20', duration: '3h 45m', stops: 0, price: 312, cabin: 'Economy' },
  { id: 'WF103', from: 'Colombo', to: 'Singapore', airline: 'SkyBridge', departure: '22:30', arrival: '05:10', duration: '4h 10m', stops: 0, price: 548, cabin: 'Premium Economy' },
  { id: 'WF104', from: 'Colombo', to: 'Singapore', airline: 'Lanka Air', departure: '09:00', arrival: '15:15', duration: '3h 45m', stops: 0, price: 1180, cabin: 'Business' },
  { id: 'WF201', from: 'Colombo', to: 'Sydney', airline: 'Southern Cross', departure: '01:20', arrival: '15:05', duration: '10h 15m', stops: 0, price: 486, cabin: 'Economy' },
  { id: 'WF202', from: 'Colombo', to: 'Sydney', airline: 'SkyBridge', departure: '10:45', arrival: '06:30', duration: '13h 15m', stops: 1, price: 418, cabin: 'Economy' },
  { id: 'WF203', from: 'Colombo', to: 'Sydney', airline: 'Southern Cross', departure: '23:55', arrival: '13:40', duration: '10h 15m', stops: 0, price: 2490, cabin: 'First Class' },
  { id: 'WF301', from: 'Colombo', to: 'London', airline: 'Atlantic Way', departure: '02:10', arrival: '09:05', duration: '11h 25m', stops: 0, price: 538, cabin: 'Economy' },
  { id: 'WF302', from: 'Colombo', to: 'London', airline: 'SkyBridge', departure: '14:20', arrival: '05:50', duration: '15h 00m', stops: 1, price: 455, cabin: 'Economy' },
  { id: 'WF303', from: 'Colombo', to: 'London', airline: 'Atlantic Way', departure: '20:40', arrival: '03:35', duration: '11h 25m', stops: 0, price: 2950, cabin: 'Business' },
  { id: 'WF401', from: 'Singapore', to: 'Sydney', airline: 'Southern Cross', departure: '07:30', arrival: '17:45', duration: '8h 15m', stops: 0, price: 296, cabin: 'Economy' },
  { id: 'WF402', from: 'Singapore', to: 'Sydney', airline: 'SkyBridge', departure: '19:10', arrival: '05:20', duration: '8h 10m', stops: 0, price: 575, cabin: 'Premium Economy' },
  { id: 'WF501', from: 'London', to: 'New York', airline: 'Atlantic Way', departure: '09:40', arrival: '12:35', duration: '7h 55m', stops: 0, price: 389, cabin: 'Economy' },
  { id: 'WF502', from: 'London', to: 'New York', airline: 'Northstar Air', departure: '16:15', arrival: '19:20', duration: '8h 05m', stops: 0, price: 1640, cabin: 'Business' },
  { id: 'WF601', from: 'Sydney', to: 'Melbourne', airline: 'Southern Cross', departure: '07:00', arrival: '08:35', duration: '1h 35m', stops: 0, price: 129, cabin: 'Economy' },
  { id: 'WF602', from: 'Sydney', to: 'Melbourne', airline: 'Southern Cross', departure: '18:20', arrival: '19:55', duration: '1h 35m', stops: 0, price: 342, cabin: 'Business' },
];

export default FLIGHTS;

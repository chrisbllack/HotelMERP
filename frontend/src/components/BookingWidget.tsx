import React, { useState } from 'react';

interface BookingForm {
  checkIn: string;
  checkOut: string;
  guests: number;
  roomType: string;
}

const BookingWidget: React.FC = () => {
  const [formData, setFormData] = useState<BookingForm>({
    checkIn: '',
    checkOut: '',
    guests: 1,
    roomType: 'standard'
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: name === 'guests' ? parseInt(value) : value
    }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // Handle booking submission
    console.log('Booking submitted:', formData);
    
    // Would typically call API here
    // await fetch('/api/bookings', { method: 'POST', body: JSON.stringify(formData) });
  };

  return (
    <div className="booking-widget">
      <h2>Book Your Stay</h2>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Check-in Date:</label>
          <input
            type="date"
            name="checkIn"
            value={formData.checkIn}
            onChange={handleChange}
            required
          />
        </div>
        
        <div>
          <label>Check-out Date:</label>
          <input
            type="date"
            name="checkOut"
            value={formData.checkOut}
            onChange={handleChange}
            required
          />
        </div>
        
        <div>
          <label>Guests:</label>
          <select
            name="guests"
            value={formData.guests}
            onChange={handleChange}
          >
            {[1, 2, 3, 4, 5].map(num => (
              <option key={num} value={num}>{num} {num === 1 ? 'Guest' : 'Guests'}</option>
            ))}
          </select>
        </div>
        
        <div>
          <label>Room Type:</label>
          <select
            name="roomType"
            value={formData.roomType}
            onChange={handleChange}
          >
            <option value="standard">Standard Room</option>
            <option value="deluxe">Deluxe Suite</option>
            <option value="suite">Executive Suite</option>
          </select>
        </div>
        
        <button type="submit">Check Availability</button>
      </form>
    </div>
  );
};

export default BookingWidget;

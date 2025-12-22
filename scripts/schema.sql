-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- USERS Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- EVENTS Table
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    total_seats INT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- SEATS Table
-- Includes 'version' column for Optimistic Locking
CREATE TYPE seat_status AS ENUM ('AVAILABLE', 'LOCKED', 'RESERVED');

CREATE TABLE IF NOT EXISTS seats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    seat_number VARCHAR(50) NOT NULL,
    category VARCHAR(50) DEFAULT 'STANDARD',
    status seat_status DEFAULT 'AVAILABLE',
    version INT DEFAULT 0, -- Optimistic Lock Version
    reserved_by UUID, -- Can be null, references users(id)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id, seat_number)
);

-- RESERVATIONS Table
CREATE TYPE reservation_status AS ENUM ('PENDING', 'PAID', 'CANCELLED');

CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    seat_id UUID NOT NULL REFERENCES seats(id),
    event_id UUID NOT NULL REFERENCES events(id),
    status reservation_status DEFAULT 'PENDING',
    amount DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for performance
CREATE INDEX idx_seats_event_status ON seats(event_id, status);
CREATE INDEX idx_reservations_user ON reservations(user_id);
CREATE INDEX idx_reservations_status ON reservations(status);

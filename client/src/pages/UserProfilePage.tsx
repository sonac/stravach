import React from 'react';
import { useParams } from 'react-router-dom';

const UserProfilePage: React.FC = () => {
  const { userId } = useParams<{ userId: string }>();

  return (
    <div style={{ padding: '20px', fontFamily: 'Arial, sans-serif' }}>
      <h1>User Profile</h1>
      <p>Details for user ID: {userId}</p>
      {/* TODO: Fetch and display user-specific data and activities */}
    </div>
  );
};

export default UserProfilePage;

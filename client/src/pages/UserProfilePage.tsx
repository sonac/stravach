import React from 'react';
import { useParams, Link } from 'react-router-dom';

const UserProfilePage: React.FC = () => {
  const { userId } = useParams<{ userId: string }>();

  return (
    <div className="flex flex-col items-center min-h-screen bg-gradient-to-br from-blue-50 to-blue-200 py-12 px-4">
      <div className="bg-white shadow-lg rounded-xl p-8 w-full max-w-md text-center">
        <img
          src={`https://api.dicebear.com/7.x/identicon/svg?seed=${userId}`}
          alt="User Avatar"
          className="w-24 h-24 mx-auto rounded-full mb-4 border-4 border-blue-300"
        />
        <h1 className="text-3xl font-bold text-blue-700 mb-2">User Profile</h1>
        <p className="text-gray-600 mb-6">Details for user ID: <span className="font-mono text-blue-900">{userId}</span></p>
        <Link
          to={`/activities/${userId}`}
          className="inline-block bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-lg shadow transition duration-200"
        >
          View Activities
        </Link>
      </div>
    </div>
  );
};

export default UserProfilePage;

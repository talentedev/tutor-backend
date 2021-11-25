var res = db.getCollection("users").updateMany(
    { "tutoring.promote_video_allowed": { $exists: false }, role: 4},
    { $set: { "tutoring.promote_video_allowed": true}}
);
print(JSON.stringify(res));

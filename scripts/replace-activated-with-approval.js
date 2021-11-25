// sets approval state for tutors

var count = 0;
db.getCollection("users").find({
    "approval": { "$exists": false },
    "activated": { "$exists": true },
}).forEach(function(user) {
    if (user.activated) {
        user.approval = 1;
    } else {
        user.approval = 0;
    }
    db.getCollection("users").save(user);
    count++;
})
print('Modified', count, 'users');
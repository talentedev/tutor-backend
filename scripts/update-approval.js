/* add 1 to approval status so as not to use 0 value
ex. New from 0 to 1, Approved from 1 to 2
RUN ONCE
 */

var count = 0;
db.getCollection("users").find({
    "approval": { "$exists": true },
}).forEach(function(user) {
    user.approval += 1;
    db.getCollection("users").save(user);
    count++;
})
print('Modified', count, 'users');
/**
 * Updates documents in collection users.tutoring.degrees[].university from ObjectId to its string value from universities.name
 * In terminal execute `mongo mongodb://localhost:port/db_name users.convert.js`
 * or
 * In mongo shell `load('users.convert.js')`
 */
uniColl = db.getCollection('universities');
function getName(id) {
    return uniColl.findOne({_id: id})
}

users = db.getCollection('users');
cursor = users.find(
    { "tutoring.degrees": { $exists: true }}
);
var allusers = cursor.toArray();
var count = 0
for(var user of allusers) {
    var degrees = user.tutoring.degrees;
    if (degrees.length > 0) {
        var hasupdate = false
        for (var degree of degrees) {
            if (typeof degree.university == "object") {
                var uni = getName(degree.university)
                if (uni == null) {
                    print('Unable to find university with id ' + degree.university + ' of user ' + user._id)
                } else {
                    degree.university = uni.name
                    hasupdate = true
                }
            }
        }
        if (hasupdate) {
            count++
            users.updateOne({_id: user._id}, {$set: {"tutoring.degrees": degrees}})
        }
    }
}
print("Updated", count)